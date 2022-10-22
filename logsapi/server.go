package logsapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
)

// LogProcessor implements client logic to process and store log messages.
type LogProcessor interface {
	// Process stores log message in persistent storage or accumulate in a buffer and flush periodically.
	Process(ctx context.Context, msg Log) error
	// Shutdown is called before exiting the extension.
	// LogProcessor should flush all the buffered data to persistent storage if any and cleanup all used resources.
	Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error
}

type options struct {
	log             logr.Logger
	logTypes        []extapi.LogSubscriptionType
	bufferingCfg    *extapi.LogsBufferingCfg
	clientOptions   []extapi.Option
	destinationAddr string
}

type loggerOption struct {
	log logr.Logger
}

func (o loggerOption) apply(opts *options) {
	opts.log = o.log
}

func WithLogger(log logr.Logger) Option {
	return loggerOption{log}
}

type Option interface {
	apply(*options)
}

type logTypesOption []extapi.LogSubscriptionType

func (o logTypesOption) apply(opts *options) {
	opts.logTypes = o
}

func WithLogTypes(types []extapi.LogSubscriptionType) Option {
	return logTypesOption(types)
}

type bufferingCfgOption struct {
	bufferingCfg *extapi.LogsBufferingCfg
}

func (o bufferingCfgOption) apply(opts *options) {
	opts.bufferingCfg = o.bufferingCfg
}

func WithBufferingCfg(bufferingCfg *extapi.LogsBufferingCfg) Option {
	return bufferingCfgOption{bufferingCfg}
}

type clientOptionsOption struct {
	clientOptions []extapi.Option
}

func (o clientOptionsOption) apply(opts *options) {
	opts.clientOptions = o.clientOptions
}

func WithClientOptionsOption(clientOptions []extapi.Option) Option {
	return clientOptionsOption{clientOptions}
}

type destinationAddrOption string

func (o destinationAddrOption) apply(opts *options) {
	opts.destinationAddr = string(o)
}

// WithDestinationAddr configures host and port for logs receiving HTTP server to listen
// Lambda API accepts only "sandbox.localdomain" host.
func WithDestinationAddr(addr string) Option {
	return destinationAddrOption(addr)
}

type extension struct {
	lp           LogProcessor
	srv          *http.Server
	logsCh       chan Log
	errCh        chan error
	doneCh       chan struct{}
	decodeCancel context.CancelFunc
	log          logr.Logger
	logTypes     []extapi.LogSubscriptionType
	bufferingCfg *extapi.LogsBufferingCfg
}

// Run runs the LogProcessor.
// Run blocks the current goroutine till extension lifecycle is finished or error occurs.
func Run(ctx context.Context, lp LogProcessor, opts ...Option) error {
	options := options{
		destinationAddr: "sandbox.localdomain:0",
		log:             logr.FromContextOrDiscard(ctx),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	decodeCtx, decodeCancel := context.WithCancel(ctx)
	ext := &extension{
		lp,
		&http.Server{
			Addr: options.destinationAddr,
			BaseContext: func(_ net.Listener) context.Context {
				return decodeCtx
			},
			ReadHeaderTimeout: time.Second,
		},
		make(chan Log),
		make(chan error, 1),
		make(chan struct{}),
		decodeCancel,
		options.log,
		options.logTypes,
		options.bufferingCfg,
	}
	ext.srv.Handler = ext

	// subscribe only to shutdown events
	options.clientOptions = append(options.clientOptions, extapi.WithEventTypes([]extapi.EventType{extapi.Shutdown}))
	// pass current logger to Extension. It will be overridden with logger from WithClientOptionsOption if passed.
	options.clientOptions = append([]extapi.Option{extapi.WithLogger(options.log)}, options.clientOptions...)
	ext.log.V(1).Info("starting extension")

	return extapi.Run(ctx, ext, options.clientOptions...)
}

func (ext *extension) Init(ctx context.Context, client *extapi.Client) error {
	go ext.startLogProcessing(ctx)

	ext.log.V(1).Info("starting log receiving HTTP server")
	ln, err := net.Listen("tcp", ext.srv.Addr)
	if err != nil {
		return fmt.Errorf("could not start log receiving HTTP server: %w", err)
	}

	go func() {
		err := ext.srv.Serve(ln)
		if !errors.Is(err, http.ErrServerClosed) {
			err = fmt.Errorf("log receiving HTTP server failed: %w", err)
			ext.log.Error(err, "")
			select {
			case ext.errCh <- err:
			default:
			}
		} else {
			ext.log.V(1).Info("log receiving HTTP server stopped")
		}
	}()

	// subscribe to lambda logs
	url := "http://" + ln.Addr().String()
	ext.log.V(1).Info(
		"calling Client.LogsSubscribe",
		"url", url,
		"logTypes", ext.logTypes,
		"bufferingCfg", ext.bufferingCfg,
	)
	req := extapi.NewLogsSubscribeRequest(url, ext.logTypes, ext.bufferingCfg)

	return client.LogsSubscribe(ctx, req)
}

func (ext *extension) HandleInvokeEvent(ctx context.Context, event *extapi.NextEventResponse) error {
	panic("unexpected HandleInvokeEvent call. Logs subscriber extension supports only Shutdown events")
}

func (ext *extension) Shutdown(ctx context.Context, reason extapi.ShutdownReason, err error) error {
	// cancel Decode context to make all in-flight and new handlers exit
	// to prevent srv.Shutdown indefinitely waiting
	ext.log.V(1).Info("signaling in-flight decode requests to stop")
	ext.decodeCancel()

	// gracefully shut down logs receiver http extension
	// shutdown server to make sure there are no writes to the channel
	ext.log.V(1).Info("shutting down HTTP server")
	srvErr := ext.srv.Shutdown(ctx)
	if srvErr != nil {
		srvErr = fmt.Errorf("could not gravefully shut down logs receiving HTTP server: %w", srvErr)
		ext.log.Error(err, "")
	}

	// after srv.Shutdown finished there are no more writers to logsCh and it can be safely closed
	// close the channel to make sure all logs are persisted
	ext.log.V(1).Info("signaling log processing to stop")
	close(ext.logsCh)

	// wait LogProcessor.Process to finish
	<-ext.doneCh

	ext.log.V(1).Info("calling LogProcessor.Shutdown")
	lpErr := ext.lp.Shutdown(ctx, reason, err)
	if lpErr != nil {
		lpErr = fmt.Errorf("LogProcessor.Shutdown failed: %w", lpErr)
		ext.log.Error(lpErr, "")

		return lpErr
	}

	return srvErr
}

func (ext *extension) Err() <-chan error {
	return ext.errCh
}

func (ext *extension) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		"received logs HTTP request. Starting decoding",
		"bytes", r.Header.Get("Content-Length"),
		"sequenceID", sequenceID,
	)
	if err := DecodeLogs(r.Context(), r.Body, ext.logsCh); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		err = fmt.Errorf("DecodeLogs failed or interrupted: %w", err)
		ext.log.Error(err, "", "sequenceID", sequenceID)
		select {
		case ext.errCh <- err:
		default:
		}

		return
	}
	ext.log.V(1).Info("logs decoding finished", "sequenceID", sequenceID)
	w.WriteHeader(http.StatusOK)
}

func (ext *extension) startLogProcessing(ctx context.Context) {
	for msg := range ext.logsCh {
		ext.log.V(1).Info("calling LogProcessor.Process", "logType", msg.LogType, "time", msg.Time)
		if err := ext.lp.Process(ctx, msg); err != nil {
			err = fmt.Errorf("LogProcessor.Process failed: %w", err)
			ext.log.Error(err, "")
			select {
			case ext.errCh <- err:
			default:
			}

			break
		}
	}

	ext.log.V(1).Info("log processing stopped")
	close(ext.doneCh)
}
