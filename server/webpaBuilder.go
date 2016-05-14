package server

import (
	"fmt"
	"github.com/Comcast/webpa-common/health"
	"github.com/Comcast/webpa-common/logging"
	"net/http"
	_ "net/http/pprof"
	"time"
)

// WebPABuilder implements the instantiation logic for each WebPA server component.
// This builder type is the standard way to construct and start a WebPA server.
type WebPABuilder struct {
	// LoggerFactory is used to create logging.Logger objects for use in
	// each server
	LoggerFactory logging.LoggerFactory

	// Configuration is the parsed configuration data, normaly from a JSON configuration file
	Configuration *Configuration

	// PrimaryHandler is the http.Handler used for the primary server
	PrimaryHandler http.Handler

	// PprofHandler is the optional handler for pprof traffic.  If omitted, http.DefaultServeMux
	// will be used instead
	PprofHandler http.Handler

	// HealthOptions define what health stats this server exposes for tracking
	HealthOptions []health.Option
}

// PrimaryAddress returns the listen address for the primary server, i.e.
// the server that listens on c.Port.
func (builder *WebPABuilder) PrimaryAddress() string {
	port := builder.Configuration.Port
	if port < 1 {
		port = DefaultPort
	}

	return fmt.Sprintf(":%d", port)
}

// HealthAddress returns the listen address for the health server
func (builder *WebPABuilder) HealthAddress() string {
	port := builder.Configuration.HealthCheckPort
	if port < 1 {
		port = DefaultHealthCheckPort
	}

	return fmt.Sprintf(":%d", port)
}

// HealthCheckInterval returns the health check interval as
// a time.Duration, using the default if c.HCInterval is nonpositive.
func (builder *WebPABuilder) HealthCheckInterval() time.Duration {
	if builder.Configuration.HealthCheckInterval < 1 {
		return DefaultHealthCheckInterval
	} else {
		return time.Duration(builder.Configuration.HealthCheckInterval)
	}
}

// PprofAddress returns the listen address for the pprof server
func (builder *WebPABuilder) PprofAddress() string {
	port := builder.Configuration.PprofPort
	if port < 1 {
		port = DefaultPprofPort
	}

	return fmt.Sprintf(":%d", port)
}

// BuildPrimary returns a Runnable that will execute the primary server
func (builder *WebPABuilder) BuildPrimary() (Runnable, error) {
	name := builder.Configuration.ServerName
	address := builder.PrimaryAddress()
	logger, err := builder.LoggerFactory.NewLogger(name)
	if err != nil {
		return nil, err
	}

	return &webPA{
		name:            name,
		address:         address,
		logger:          logger,
		certificateFile: builder.Configuration.CertificateFile,
		keyFile:         builder.Configuration.KeyFile,
		serverExecutor: &http.Server{
			Addr:      address,
			Handler:   builder.PrimaryHandler,
			ConnState: logging.NewConnectionStateLogger(logger, name),
			ErrorLog:  logging.NewErrorLog(logger, name),
		},
	}, nil
}

// BuildHealth is a factory function for both the WebPA server that exposes health statistics
// and the underlying Health object, both of which are Runnable.
func (builder *WebPABuilder) BuildHealth() (Runnable, error) {
	name := builder.Configuration.ServerName + healthSuffix
	address := builder.HealthAddress()
	logger, err := builder.LoggerFactory.NewLogger(name)
	if err != nil {
		return nil, err
	}

	var runnables [2]Runnable
	healthHandler := health.New(builder.HealthCheckInterval(), logger, builder.HealthOptions...)
	runnables[0] = healthHandler

	runnables[1] = &webPA{
		name:    name,
		address: address,
		logger:  logger,
		serverExecutor: &http.Server{
			Addr:      address,
			Handler:   healthHandler,
			ConnState: logging.NewConnectionStateLogger(logger, name),
			ErrorLog:  logging.NewErrorLog(logger, name),
		},
	}

	return RunnableSet(runnables[0:2]), nil
}

// BuildPprof is a factory function for the pprof server defined in the configuration
func (builder *WebPABuilder) BuildPprof() (Runnable, error) {
	name := builder.Configuration.ServerName + pprofSuffix
	address := builder.PprofAddress()
	logger, err := builder.LoggerFactory.NewLogger(name)
	if err != nil {
		return nil, err
	}

	pprofHandler := builder.PprofHandler
	if pprofHandler == nil {
		// by default, net/http/pprof registers the handles
		// using the default server mux
		pprofHandler = http.DefaultServeMux
	}

	return &webPA{
		name:    name,
		address: address,
		logger:  logger,
		serverExecutor: &http.Server{
			Addr:      address,
			Handler:   pprofHandler,
			ConnState: logging.NewConnectionStateLogger(logger, name),
			ErrorLog:  logging.NewErrorLog(logger, name),
		},
	}, nil
}

// BuildAll returns a single Runnable that executes all server components produced
// by this builder: pprof, health, and the primary server
func (builder *WebPABuilder) BuildAll() (runnable Runnable, err error) {
	var runnables [3]Runnable
	runnables[0], err = builder.BuildPprof()
	if err != nil {
		return
	}

	runnables[1], err = builder.BuildHealth()
	if err != nil {
		return
	}

	runnables[2], err = builder.BuildPrimary()
	if err != nil {
		return
	}

	runnable = RunnableSet(runnables[0:3])
	return
}