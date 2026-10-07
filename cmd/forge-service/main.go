package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"manatomb/app/internal/forge"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func duration(k, d string) time.Duration {
	v, err := time.ParseDuration(env(k, d))
	if err != nil || v <= 0 {
		log.Fatalf("%s must be a positive duration", k)
	}
	return v
}
func main() {
	cap, err := strconv.Atoi(env("FORGE_MAX_GAMES", "1"))
	if err != nil {
		log.Fatal(err)
	}
	identity, err := os.ReadFile(filepath.Join(env("FORGE_BUNDLE", "/opt/forge"), "identity.json"))
	if err != nil {
		log.Fatal("Forge bundle identity is missing: ", err)
	}
	build := fmt.Sprintf("%x", sha256.Sum256(identity))
	s, err := forge.NewService(forge.ServiceConfig{
		Build: build, Secret: os.Getenv("FORGE_SERVICE_SECRET"), Launcher: env("FORGE_LAUNCHER", "/opt/forge/launch"), Runtime: env("FORGE_SESSIONS_DIR", "/tmp/manatomb-games"), Defaults: env("FORGE_DEFAULTS_DIR", "/opt/forge/defaults"), Cap: cap,
		Disconnect: duration("FORGE_DISCONNECT_TIMEOUT", "5m"), Idle: duration("FORGE_IDLE_TIMEOUT", "15m"),
		NoPrompt: duration("FORGE_NO_PROMPT_TIMEOUT", "90s"),
		Maximum:  duration("FORGE_MAX_SESSION", "2h"), Startup: duration("FORGE_STARTUP_TIMEOUT", "3m"),
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: ":" + env("PORT", "8081"), Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
		close(done)
	}()
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Print(err)
		stop()
	}
	<-done
}
