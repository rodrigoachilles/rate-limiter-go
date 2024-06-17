package main

import (
	"context"
	"fmt"
	"github.com/rodrigoachilles/rate-limiter/configs"
	"github.com/rodrigoachilles/rate-limiter/configs/logger"
	"github.com/rodrigoachilles/rate-limiter/internal/infra/middleware"
	repository "github.com/rodrigoachilles/rate-limiter/internal/infra/redis"
	"github.com/rodrigoachilles/rate-limiter/internal/usecase/limiter"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	if err := run(); err != nil {
		logger.Fatal(err.Error(), err)
	}
}

func run() (err error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := configs.LoadConfig()
	if err != nil {
		logger.Fatal("failed to load config", err)
	}

	srv := &http.Server{
		Addr:         cfg.ServerPort,
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		Handler:      handler(cfg.RedisAddr, cfg.IPLimit, cfg.TokenLimit, cfg.BlockTime),
	}
	srvErr := make(chan error, 1)

	go func() {
		logger.Info(fmt.Sprintf("Starting server on port '%s'...", cfg.ServerPort[1:]))
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err = <-srvErr:
		return
	case <-ctx.Done():
		stop()
	}

	err = srv.Shutdown(context.Background())
	return
}

func handler(redisAddr string, ipLimit, tokenLimit int, blockTime time.Duration) http.Handler {
	rdb := repository.NewRedisClient(redisAddr)
	l := limiter.NewLimiter(rdb, ipLimit, tokenLimit, blockTime)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Rate limiter!"))
	})

	return middleware.RateLimiter(l)(mux)
}
