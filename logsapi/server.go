package logsapi

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/zakharovvi/aws-lambda-extensions/extapi"
	"github.com/zakharovvi/aws-lambda-extensions/internal"
)

// Processor implements client logic to process and store log messages.
type Processor interface {
	// Init is called before starting receiving logs and Process.
	// It's the best place to make network connections, warmup caches, preallocate buffers, etc.
	Init(ctx context.Context, registerResp *extapi.RegisterResponse) error
	// Process stores log message in persistent storage or accumulate in a buffer and flush periodically.
	Process(ctx context.Context, event Log) error
	// Shutdown is called before exiting the extension.
	// Processor should flush all the buffered data to persistent storage if any and cleanup all used resources.
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

// Run runs the Processor.
// Run blocks the current goroutine till extension lifecycle is finished or error occurs.
func Run(ctx context.Context, proc Processor, opts ...Option) error {
	options := options{
		destinationAddr: "sandbox.localdomain:0",
		log:             logr.FromContextOrDiscard(ctx),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	subscriber := func(ctx context.Context, client *extapi.Client, destinationURL string) error {
		options.log.V(1).Info(
			"calling Client.LogsSubscribe",
			"url", destinationURL,
			"logTypes", options.logTypes,
			"bufferingCfg", options.bufferingCfg,
		)
		req := extapi.NewLogsSubscribeRequest(destinationURL, options.logTypes, options.bufferingCfg)

		return client.LogsSubscribe(ctx, req)
	}

	ext := internal.NewExtension[Log](
		ctx,
		proc,
		options.destinationAddr,
		options.log,
		DecodeLogs,
		subscriber,
	)

	// subscribe only to shutdown events
	options.clientOptions = append(options.clientOptions, extapi.WithEventTypes([]extapi.EventType{extapi.Shutdown}))
	// pass current logger to Extension. It will be overridden with logger from WithClientOptionsOption if passed.
	options.clientOptions = append([]extapi.Option{extapi.WithLogger(options.log)}, options.clientOptions...)
	options.log.V(1).Info("starting extension")

	return extapi.Run(ctx, ext, options.clientOptions...)
}
