package extapi

import (
	"context"
	"fmt"
	"time"
)

// Extension abstracts the extension logic from Lambda Extensions API.
// For Telemetry API extension, use telemetryapi.Processor and telemetryapi.Run.
type Extension interface {
	// Init is called after extension Register and before invoking lambda function.
	// It's the best place to make network connections, warmup caches, preallocate buffers, etc.
	Init(ctx context.Context, client *Client) error
	// HandleInvokeEvent is called after receiving Invoke event type from Lambda API.
	// Shutdown event type is handled inside Run internally and not exposed to the Extension.
	HandleInvokeEvent(ctx context.Context, event *NextEventResponse) error
	// Shutdown is called when Lambda API signals the extension to stop or in case of an error.
	// There will be no calls of HandleInvokeEvent after Shutdown was called.
	// Extension should flush all unsaved changes to persistent storage.
	// Run will return after calling the Shutdown and handling its result.
	Shutdown(ctx context.Context, reason ShutdownReason, err error) error
	// Err signals an error to Run loop and stop the extension.
	// Only the first error is read from the channel. Consider using unblocking send to put errors into the channel.
	// error channel can be nil.
	Err() <-chan error
}

// Run runs the Extension.
// Run blocks the current goroutine till extension lifecycle is finished or error occurs.
func Run(ctx context.Context, ext Extension, opts ...Option) error {
	client, registerErr := Register(ctx, opts...)
	if registerErr != nil {
		return registerErr
	}
	log := client.log

	log.V(1).Info("calling Extension.Init")
	if initErr := ext.Init(ctx, client); initErr != nil {
		log.Error(initErr, "Extension.Init failed")
		if _, err := client.InitError(ctx, "Extension.Init", initErr); err != nil {
			log.Error(err, "client.InitError failed")
		}
		log.V(1).Info("calling Extension.Shutdown")
		if err := ext.Shutdown(ctx, ExtensionError, initErr); err != nil {
			log.Error(err, "Extension.Shutdown failed")
		}

		return fmt.Errorf("Extension.Init failed: %w", initErr)
	}

	log.V(1).Info("Extension.Init completed. Starting Client.NextEvent loop")
	shutdownEvent, loopErr := loop(ctx, client, ext)
	if loopErr != nil {
		loopErr = fmt.Errorf("extension loop failed: %w", loopErr)
	}

	shutdownErr := shutdown(ctx, client, ext, shutdownEvent, loopErr)
	if loopErr != nil {
		return loopErr
	}

	return shutdownErr
}

// shutdown calls Extension.Shutdown and report an error to Client.ExitError if any.
func shutdown(ctx context.Context, client *Client, ext Extension, event *NextEventResponse, err error) error {
	reason := ExtensionError
	if event != nil {
		reason = event.ShutdownReason

		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, time.UnixMilli(event.DeadlineMs))
		defer cancel()
	}

	client.log.V(1).Info("calling Extension.Shutdown")
	shutdownErr := ext.Shutdown(ctx, reason, err)
	if shutdownErr != nil {
		shutdownErr = fmt.Errorf("Extension.Shutdown failed: %w", shutdownErr)
		client.log.Error(shutdownErr, "")
		if err == nil {
			err = shutdownErr
		}
	}

	if err != nil {
		client.log.V(1).Info("calling Client.ExitError", "err", err)
		if _, err := client.ExitError(ctx, "Extension.Exit", err); err != nil {
			client.log.Error(err, "Client.ExitError error failed")
		}
	}

	return shutdownErr
}

// loop polls Client.NextEvent and blocks until Shutdown event received, error occurs, or context cancelled.
func loop(ctx context.Context, client *Client, ext Extension) (*NextEventResponse, error) {
	defer client.log.V(1).Info("Client.NextEvent loop stopped")
	nextEventCh := make(chan *NextEventResponse)
	nextEventErrCh := make(chan error)

	// cleanup Client.NextEvent goroutine in case of external error received
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		// run Client.NextEvent in a separate goroutine instead of select's default,
		// as it can block for a long time inside frozen execution environment
		// or if extension is subscribed only to Shutdown event
		go func() {
			client.log.V(1).Info("calling Client.NextEvent")
			event, err := client.NextEvent(ctx)
			if err != nil {
				nextEventErrCh <- err
			} else {
				nextEventCh <- event
			}
		}()

		select {
		case event := <-nextEventCh:
			if event.EventType == Shutdown {
				client.log.Info("shutdown event received", "event", event)

				return event, nil
			}

			client.log.V(1).Info("calling Extension.HandleInvokeEvent", "event", event)
			handleCtx, handleCancel := context.WithDeadline(ctx, time.UnixMilli(event.DeadlineMs))
			err := ext.HandleInvokeEvent(handleCtx, event)
			handleCancel()

			if err != nil {
				return nil, fmt.Errorf("Extension.HandleInvokeEvent failed: %w", err)
			}
		case err := <-nextEventErrCh:
			return nil, fmt.Errorf("Client.NextEvent failed: %w", err)
		case err := <-ext.Err():
			return nil, fmt.Errorf("Extension.Err() signaled an error: %w", err)
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled before calling Client.NextEvent: %w", ctx.Err())
		}
	}
}
