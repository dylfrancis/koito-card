package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dylfrancis/koito-card/internal/koito"
	"github.com/dylfrancis/koito-card/internal/server"
)

// version is set at build time with -ldflags.
var version = "dev"

func main() {
	port := envOr("PORT", "8080")
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck(port))
	}

	koitoAddress := os.Getenv("KOITO_URL")
	koitoURL, err := url.Parse(koitoAddress)
	if koitoAddress == "" || err != nil || (koitoURL.Scheme != "http" && koitoURL.Scheme != "https") || koitoURL.Host == "" {
		log.Fatal("KOITO_URL must be the address of your Koito server, like http://koito:4110")
	}
	cacheSeconds, err := strconv.Atoi(envOr("CACHE_SECONDS", "300"))
	if err != nil || cacheSeconds < 0 {
		log.Fatal("CACHE_SECONDS must be a whole number of seconds")
	}

	client := koito.New(koitoAddress, time.Duration(cacheSeconds)*time.Second, "koito-card/"+version)
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           logRequests(server.New(client, cacheSeconds)),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("koito-card %s listening on :%s, reading %s", version, port, koitoURL.Redacted())
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// healthcheck backs Docker's HEALTHCHECK, since the image has no shell or curl.
func healthcheck(port string) (exitCode int) {
	client := http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if r.URL.Path != "/healthz" {
			log.Printf("%s %s %d %s", r.Method, r.URL.RequestURI(), recorder.status, time.Since(started).Round(time.Millisecond))
		}
	})
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
