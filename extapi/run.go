package extapi

import (
	"context"
	"time"
)

type Extension interface {
	Init(ctx context.Context, client *Client) error
	Invoke(ctx context.Context, event *NextEventResponse) error
	Shutdown(ctx context.Context) error
}

func Run(ctx context.Context, ext Extension, opts ...Option) error {
	client, err := Register(ctx, opts...)
	if err != nil {
		return err
	}

	if err := ext.Init(ctx, client); err != nil {
		_, _ = client.InitError(ctx, "ExtensionRun.Init", err)
		_ = ext.Shutdown(ctx)
		return err
	}

	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			break
		default:
		}

		var event *NextEventResponse
		event, err = client.NextEvent(ctx)
		if err != nil {
			break
		}

		select {
		case <-ctx.Done():
			err = ctx.Err()
			break
		default:
		}

		if event.EventType == Shutdown {
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(ctx, time.UnixMilli(event.DeadlineMs))
			defer cancel()
			break
		}

		handleCtx, cancel := context.WithDeadline(ctx, time.UnixMilli(event.DeadlineMs))
		err = ext.Invoke(handleCtx, event)
		cancel()
		if err != nil {
			break
		}
	}

	if err != nil {
		_, _ = client.ExitError(ctx, "ExtensionRun.Invoke", err)
	}
	_ = ext.Shutdown(ctx)
	return err
}
