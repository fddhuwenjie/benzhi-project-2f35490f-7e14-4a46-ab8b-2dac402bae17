package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shelter-drill-gate/internal/application"
	"shelter-drill-gate/internal/storage"
	"shelter-drill-gate/internal/web"
)

type runtime struct {
	store    *storage.Store
	server   *http.Server
	listener net.Listener
}

func buildRuntime(cfg config) (*runtime, error) {
	store, err := storage.Open(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("初始化 SQLite: %w", err)
	}
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("监听 %s: %w", cfg.Addr, err)
	}
	service := application.NewService(store)
	handler := web.NewServer(service).Handler()
	server := &http.Server{Handler: handler, ReadHeaderTimeout: readHeaderTimeout, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	return &runtime{store: store, server: server, listener: listener}, nil
}

func (r *runtime) serve() <-chan error {
	done := make(chan error, 1)
	go func() {
		err := r.server.Serve(r.listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		done <- err
	}()
	return done
}

func (r *runtime) close(ctx context.Context) error {
	serverErr := r.server.Shutdown(ctx)
	storeErr := r.store.Close()
	if serverErr != nil {
		return serverErr
	}
	return storeErr
}

func runServer(cfg config) error {
	runtime, err := buildRuntime(cfg)
	if err != nil {
		return err
	}
	done := runtime.serve()
	log.Printf("避难场所演练核验台已监听 http://%s", runtime.listener.Addr())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	select {
	case err := <-done:
		_ = runtime.store.Close()
		return err
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := runtime.close(ctx); err != nil {
			return fmt.Errorf("关闭服务: %w", err)
		}
		return <-done
	}
}
