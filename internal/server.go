package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

type eventProcessor[T any] interface {
	Init(ctx context.Context, client *extapi.Client) error
	Process(ctx context.Context, event T) error
	Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error
}

type decoder[T any] func(ctx context.Context, r io.ReadCloser, events chan<- T) error

type subscriber func(ctx context.Context, client *extapi.Client, destinationURL string) error

type Extension[T any] struct {
	ep           eventProcessor[T]
	srv          *http.Server
	eventsCh     chan T
	errCh        chan error
	doneCh       chan struct{}
	decodeCancel context.CancelFunc
	log          logr.Logger
	decoder      decoder[T]
	subscriber   subscriber
}

func NewExtension[T any](
	ctx context.Context,
	lp eventProcessor[T],
	destinationAddr string,
	log logr.Logger,
	decoder decoder[T],
	subscriber subscriber,
) *Extension[T] {
	decodeCtx, decodeCancel := context.WithCancel(ctx)
	ext := &Extension[T]{
		lp,
		&http.Server{
			Addr: destinationAddr,
			BaseContext: func(_ net.Listener) context.Context {
				return decodeCtx
			},
			ReadHeaderTimeout: time.Second,
		},
		make(chan T),
		make(chan error, 1),
		make(chan struct{}),
		decodeCancel,
		log,
		decoder,
		subscriber,
	}
	ext.srv.Handler = ext
	return ext
}

func (ext *Extension[T]) Init(ctx context.Context, client *extapi.Client) error {
	// start log processing goroutine before EventProcessor.Init().
	// in case of Init error ext.Shutdown is called and waits for ext.doneCh to be closed in ext.startEventProcessing
	go ext.startEventProcessing(ctx)

	if err := ext.ep.Init(ctx, client); err != nil {
		return fmt.Errorf("EventProcessor.Init failed: %w", err)
	}

	ext.log.V(1).Info("starting event receiving HTTP server")
	ln, err := net.Listen("tcp", ext.srv.Addr)
	if err != nil {
		return fmt.Errorf("could not start event receiving HTTP server: %w", err)
	}

	go func() {
		err := ext.srv.Serve(ln)
		if !errors.Is(err, http.ErrServerClosed) {
			err = fmt.Errorf("event receiving HTTP server failed: %w", err)
			ext.log.Error(err, "")
			select {
			case ext.errCh <- err:
			default:
			}
		} else {
			ext.log.V(1).Info("event receiving HTTP server stopped")
		}
	}()

	// subscribe to lambda event
	url, err := ext.destinationURL(ln.Addr())
	if err != nil {
		return fmt.Errorf("could not build url for subscribe API call: %w", err)
	}

	return ext.subscriber(ctx, client, url)
}

func (ext *Extension[T]) destinationURL(listenerAddr net.Addr) (string, error) {
	// we should get host from the user,
	// as host in listenerAddr is resolved to ip address which is not permitted in Lambda API
	host, _, err := net.SplitHostPort(ext.srv.Addr)
	if err != nil {
		return "", err
	}

	// if user provided port is zero we should get the actual port from the listener
	_, port, err := net.SplitHostPort(listenerAddr.String())
	if err != nil {
		return "", err
	}

	return "http://" + net.JoinHostPort(host, port), nil
}

func (ext *Extension[T]) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	panic("unexpected HandleInvokeEvent call. Events subscriber extension supports only Shutdown events")
}

func (ext *Extension[T]) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	// cancel Decode context to make all in-flight and new handlers exit
	// to prevent srv.Shutdown indefinitely waiting
	ext.log.V(1).Info("signaling in-flight decode requests to stop")
	ext.decodeCancel()

	// gracefully shut down event receiver http extension
	// shutdown server to make sure there are no writes to the channel
	ext.log.V(1).Info("shutting down HTTP server")
	srvErr := ext.srv.Shutdown(ctx)
	if srvErr != nil {
		srvErr = fmt.Errorf("could not gravefully shut down events receiving HTTP server: %w", srvErr)
		ext.log.Error(err, "")
	}

	// after srv.Shutdown finished there are no more writers to eventsCh and it can be safely closed
	// close the channel to make sure all events are persisted
	ext.log.V(1).Info("signaling event processing to stop")
	close(ext.eventsCh)

	// wait EventProcessor.Process to finish
	<-ext.doneCh

	ext.log.V(1).Info("calling EventProcessor.Shutdown")
	lpErr := ext.ep.Shutdown(ctx, reason, err)
	if lpErr != nil {
		lpErr = fmt.Errorf("EventProcessor.Shutdown failed: %w", lpErr)
		ext.log.Error(lpErr, "")

		return lpErr
	}

	return srvErr
}

func (ext *Extension[T]) Err() <-chan error {
	return ext.errCh
}

func (ext *Extension[T]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sequenceID := r.Header.Get("Sequence-Id")

	if r.Method != http.MethodPost {
		err := fmt.Errorf("got unexpected HTTP request method %s, want POST", r.Method)
		http.Error(w, err.Error(), http.StatusBadRequest)
		ext.log.Error(err, "", "sequenceID", sequenceID)
		select {
		case ext.errCh <- err:
		default:
		}

		return
	}

	ext.log.V(1).Info(
		"received events HTTP request. Starting decoding",
		"bytes", r.Header.Get("Content-Length"),
		"sequenceID", sequenceID,
	)
	if err := ext.decoder(r.Context(), r.Body, ext.eventsCh); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		err = fmt.Errorf("decoding failed or interrupted: %w", err)
		ext.log.Error(err, "", "sequenceID", sequenceID)
		select {
		case ext.errCh <- err:
		default:
		}

		return
	}
	ext.log.V(1).Info("events decoding finished", "sequenceID", sequenceID)
}

func (ext *Extension[T]) startEventProcessing(ctx context.Context) {
	for event := range ext.eventsCh {
		ext.log.V(1).Info("calling EventProcessor.Process", "event", event)
		if err := ext.ep.Process(ctx, event); err != nil {
			err = fmt.Errorf("EventProcessor.Process failed: %w", err)
			ext.log.Error(err, "")
			select {
			case ext.errCh <- err:
			default:
			}

			break
		}
	}

	ext.log.V(1).Info("event processing stopped")
	close(ext.doneCh)
}
