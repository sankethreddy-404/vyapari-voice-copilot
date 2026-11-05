package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vyapari-voice-copilot/internal/api"
	"vyapari-voice-copilot/internal/config"
	"vyapari-voice-copilot/internal/gnani"
	"vyapari-voice-copilot/internal/store"
)

func main() {
	cfg := config.Load()

	dataStore, err := store.Open(cfg.DataFile)
	if err != nil {
		log.Fatalf("open data store: %v", err)
	}
	gnaniClient := gnani.New(cfg)
	application := api.New(cfg, dataStore, gnaniClient)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       75 * time.Second,
		WriteTimeout:      75 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		log.Printf("Vyapari Voice Copilot %s", api.Version)
		log.Printf("Open http://localhost%s", cfg.Addr)
		if cfg.GnaniConfigured() {
			log.Printf("Gnani API integration: configured")
		} else {
			log.Printf("Gnani API integration: not configured; typed-command demo remains available")
		}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Printf("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
