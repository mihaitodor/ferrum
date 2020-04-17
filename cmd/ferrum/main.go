package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/mihaitodor/ferrum/config"
	"github.com/mihaitodor/ferrum/server"
	"github.com/relistan/rubberneck"

	log "github.com/sirupsen/logrus"
)

// initGracefulStop sets up a context which gets cancelled when the process
// receives and traps either SIGINT or SIGTERM
func initGracefulStop() context.Context {
	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sig := <-gracefulStop
		log.Warnf("Received signal %q. Exiting as soon as possible!", sig)
		cancel()
	}()

	return ctx
}

func main() {
	c, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Print the configuration to stdout
	rubberneck.Print(c)

	log.SetLevel(c.LogLevel)

	log.Info("Starting Ferrum server")

	s, err := server.New(c)
	if err != nil {
		log.Fatalf("Failed to initialise server: %v", err)
	}

	ctx := initGracefulStop()
	if err := s.ConnectDatabase(ctx); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	s.SetupHTTPHandlers()

	// Spin up the HTTP server
	go s.ListenAndServe()

	// Wait for shutdown signal
	<-ctx.Done()

	// Shutdown server gracefully
	ctx, done := context.WithTimeout(context.Background(), c.HTTPRequestTimeout)
	defer done()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("Ferrum server exited with error: %v", err)
	}

	log.Info("Ferrum server shut down successfully")
}
