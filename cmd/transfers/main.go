package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CXTACLYSM/saga-practice/configs"
	"github.com/CXTACLYSM/saga-practice/internal"
	router "github.com/CXTACLYSM/saga-practice/internal/transfer/infrastructure/http"
)

func main() {
	cfg := configs.NewConfig()
	container, err := internal.NewContainer(cfg)
	if err != nil {
		log.Fatalf("error initializing container: %v", err)
	}
	defer container.Infrastructure.Pool.Close()
	defer container.Infrastructure.AmqpConn.Close()

	r := router.NewRouter(container.Handlers)
	srv := http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", cfg.App.Http.Port),
		Handler:           r,
		ReadTimeout:       cfg.App.Http.ReadTimeout,
		ReadHeaderTimeout: cfg.App.Http.ReadHeaderTimeout,
		WriteTimeout:      cfg.App.Http.WriteTimeout,
		IdleTimeout:       cfg.App.Http.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err = srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			log.Fatalf("error staring http server: %v", err)
		}
	}()
	container.Workers.OutboxWorker.Start(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = srv.Shutdown(ctx); err != nil {
		log.Fatalf("error graceful shutdown http server: %v", err)
	}

	fmt.Println("server gracefully stopped")
}
