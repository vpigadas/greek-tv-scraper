package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vpigadas/greek-tv-scraper/internal/api"
	"github.com/vpigadas/greek-tv-scraper/internal/config"
	"github.com/vpigadas/greek-tv-scraper/internal/metrics"
	"github.com/vpigadas/greek-tv-scraper/internal/scheduler"
	"github.com/vpigadas/greek-tv-scraper/internal/store"
)

func main() {
	_ = godotenv.Load()
	metrics.StartTime.SetToCurrentTime()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Connect to Redis with retry (5 attempts, 2s backoff)
	redisStore := store.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.FutureScheduleTTL, cfg.PastScheduleTTL)
	for attempt := 1; attempt <= 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := redisStore.Ping(ctx)
		cancel()
		if err == nil {
			log.Printf("redis: connected to %s db=%d", cfg.RedisAddr, cfg.RedisDB)
			break
		}
		if attempt == 5 {
			log.Fatalf("redis: connection failed after %d attempts: %v", attempt, err)
		}
		log.Printf("redis: connection attempt %d/5 failed: %v — retrying in 2s", attempt, err)
		time.Sleep(2 * time.Second)
	}

	// Start scheduler
	sched := scheduler.New(cfg, redisStore)
	if err := sched.Start(); err != nil {
		log.Fatalf("scheduler: %v", err)
	}
	defer sched.Stop()

	// Start now-updater
	nowUpdater := scheduler.NewNowUpdater(redisStore, cfg.AthensLocation)
	nowUpdater.Start()
	defer nowUpdater.Stop()

	r := chi.NewRouter()
	r.Use(api.Recovery) // panic recovery — must be first
	r.Use(api.Metrics)
	r.Use(api.Logger)
	r.Use(api.CORS)

	handler := api.NewHandler(redisStore, cfg)
	handler.RegisterRoutes(r)

	r.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server: listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("server: shutting down")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("server: shutdown error: %v", err)
	}
}
