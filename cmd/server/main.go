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
	_ "github.com/vpigadas/greek-tv-scraper/internal/metrics" // register metrics
	"github.com/vpigadas/greek-tv-scraper/internal/scheduler"
	"github.com/vpigadas/greek-tv-scraper/internal/store"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	redisStore := store.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.ScheduleTTL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisStore.Ping(ctx); err != nil {
		log.Fatalf("redis: connection failed: %v", err)
	}
	log.Printf("redis: connected to %s db=%d", cfg.RedisAddr, cfg.RedisDB)

	sched := scheduler.New(cfg, redisStore)
	sched.Start()
	defer sched.Stop()

	r := chi.NewRouter()
	r.Use(api.Metrics)
	r.Use(api.Logger)
	r.Use(api.CORS)

	handler := api.NewHandler(redisStore, cfg)
	handler.RegisterRoutes(r)

	// Prometheus metrics endpoint
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
	_ = srv.Shutdown(shutCtx)
}
