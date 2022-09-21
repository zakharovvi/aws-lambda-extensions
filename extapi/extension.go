package extapi

import (
	"context"
	"fmt"
	"time"
)

type Extension interface {
	Init(ctx context.Context, client *Client) error
	HandleInvokeEvent(ctx context.Context, event *NextEventResponse) error
	Shutdown(ctx context.Context, reason ShutdownReason) error
}

func Run(ctx context.Context, ext Extension, opts ...Option) error {
	client, err := Register(ctx, opts...)
	if err != nil {
		return err
	}
	log := client.log

	log.V(1).Info("calling Extension.Init")
	if err := ext.Init(ctx, client); err != nil {
		log.Error(err, "Extension.Init failed")
		if _, err := client.InitError(ctx, "Extension.Init", err); err != nil {
			log.Error(err, "client.InitError failed")
		}
		log.V(1).Info("calling Extension.Shutdown")
		if err := ext.Shutdown(ctx, ExtensionError); err != nil {
			log.Error(err, "Extension.Shutdown failed")
		}

		return fmt.Errorf("Extension.Init failed: %w", err)
	}
	log.V(1).Info("Extension.Init completed. Starting Client.NextEvent loop")
	shutdownReason := ExtensionError
loop:
	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			log.Error(err, "context cancelled before calling Client.NextEvent")

			break loop
		default:
		}

		log.V(1).Info("calling Client.NextEvent")
		var event *NextEventResponse
		event, err = client.NextEvent(ctx)
		if err != nil {
			log.Error(err, "Client.NextEvent failed")

			break loop
		}

		select {
		case <-ctx.Done():
			err = ctx.Err()
			log.Error(err, "context cancelled after receiving Client.NextEvent result")

			break loop
		default:
		}

		if event.EventType == Shutdown {
			shutdownReason = event.ShutdownReason
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(ctx, time.UnixMilli(event.DeadlineMs))
			defer cancel()
			log.Info("shutdown event received", "event", event)

			break loop
		}

		log.V(1).Info("calling Extension.HandleInvokeEvent", "event", event)
		handleCtx, cancel := context.WithDeadline(ctx, time.UnixMilli(event.DeadlineMs))
		err = ext.HandleInvokeEvent(handleCtx, event)
		cancel()
		if err != nil {
			log.Error(err, "Extension.HandleInvokeEvent failed")

			break loop
		}
	}
	log.V(1).Info("Client.NextEvent loop stopped")
	if err != nil {
		err = fmt.Errorf("extension loop failed: %w", err)
	}

	log.V(1).Info("calling Extension.Shutdown")
	if shutdownErr := ext.Shutdown(ctx, shutdownReason); shutdownErr != nil {
		shutdownErr = fmt.Errorf("Extension.Shutdown failed: %w", shutdownErr)
		log.Error(shutdownErr, "")
		if err == nil {
			err = shutdownErr
		}
	}

	if err != nil {
		log.V(1).Info("calling Client.ExitError", "err", err)
		if _, err := client.ExitError(ctx, "Extension.Exit", err); err != nil {
			log.Error(err, "Client.ExitError error failed")
		}
	}

	return err
}
