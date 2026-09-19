package web

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/1set/starbox"
	"github.com/1set/starlet"
	shttp "github.com/1set/starlet/lib/http"
	"go.uber.org/zap"
)

// Config defines the host's HTTP admission and execution limits. All limits
// must be positive. They do not replace process isolation for untrusted code.
type Config struct {
	Address        string
	MaxBodyBytes   int64
	MaxConcurrent  int
	RequestTimeout time.Duration
}

// DefaultConfig binds loopback and gives each request bounded input and time.
func DefaultConfig(port uint16) Config {
	return Config{
		Address:        net.JoinHostPort("127.0.0.1", fmt.Sprint(port)),
		MaxBodyBytes:   1 << 20,
		MaxConcurrent:  16,
		RequestTimeout: 15 * time.Second,
	}
}

func (c Config) validate() error {
	if c.Address == "" || c.MaxBodyBytes <= 0 || c.MaxConcurrent <= 0 || c.RequestTimeout <= 0 {
		return errors.New("web: address and positive body, concurrency, and timeout limits are required")
	}
	return nil
}

func handler(builder func() *starbox.RunnerConfig) http.HandlerFunc {
	return requestHandler(DefaultConfig(0), builder)
}

func requestHandler(cfg Config, builder func() *starbox.RunnerConfig) http.HandlerFunc {
	slots := make(chan struct{}, cfg.MaxConcurrent)
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "server busy", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), cfg.RequestTimeout)
		defer cancel()
		if ctx.Err() != nil {
			http.Error(w, "request canceled", http.StatusRequestTimeout)
			return
		}
		if r.Body == nil {
			r.Body = http.NoBody
		}
		r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBodyBytes)
		req, err := shttp.NewExportedServerRequest(r)
		if err != nil {
			var limit *http.MaxBytesError
			status := http.StatusBadRequest
			if errors.As(err, &limit) {
				status = http.StatusRequestEntityTooLarge
			}
			http.Error(w, http.StatusText(status), status)
			return
		}
		resp := shttp.NewServerResponse()
		var runner *starbox.RunnerConfig
		if builder != nil {
			runner = builder()
		}
		if runner == nil {
			http.Error(w, "runtime unavailable", http.StatusInternalServerError)
			return
		}
		_, err = runner.Context(ctx).KeyValueMap(starlet.StringAnyMap{
			"request": req.Struct(), "response": resp.Struct(),
		}).Execute()
		if ctx.Err() != nil {
			status := http.StatusRequestTimeout
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				status = http.StatusGatewayTimeout
			}
			http.Error(w, http.StatusText(status), status)
			return
		}
		if err != nil {
			log.Warnw("fail to execute code", zap.Error(err))
			http.Error(w, fmt.Sprintf("Runtime Error: %v", err), http.StatusInternalServerError)
			return
		}
		if err := resp.Write(w); err != nil {
			log.Warnw("fail to write response", zap.Error(err))
		}
	}
}

func newServer(ctx context.Context, cfg Config, builder func() *starbox.RunnerConfig) *http.Server {
	return &http.Server{
		Addr: cfg.Address, Handler: requestHandler(cfg, builder),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      cfg.RequestTimeout + 10*time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
}

// Start serves trusted, host-selected code on loopback. Hosts needing graceful
// cancellation or an explicit remote bind should use StartContext.
func Start(port uint16, builder func() *starbox.RunnerConfig) error {
	return StartContext(context.Background(), DefaultConfig(port), builder)
}

// StartContext returns startup/serve errors to the host. Cancellation stops
// admission, cancels request execution, drains connections, then closes stragglers.
func StartContext(ctx context.Context, cfg Config, builder func() *starbox.RunnerConfig) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}
	return serve(ctx, listener, cfg, builder)
}

func serve(ctx context.Context, listener net.Listener, cfg Config, builder func() *starbox.RunnerConfig) error {
	server := newServer(ctx, cfg, builder)
	finished := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				_ = server.Close()
			}
		case <-finished:
		}
	}()
	log.Infow("web server started", "address", listener.Addr().String())
	err := server.Serve(listener)
	close(finished)
	<-stopped
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
