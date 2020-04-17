package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/mihaitodor/ferrum/config"
	"github.com/mihaitodor/ferrum/db"
	log "github.com/sirupsen/logrus"
)

type dbConn interface {
	db.DBTX
	Ping() error
	Close() error
}

type queries interface {
	AddPatient(context.Context, db.AddPatientParams) (db.Patient, error)
	GetPatient(context.Context, int32) (db.Patient, error)
	GetPatients(context.Context) ([]db.Patient, error)
}

// Server implements the main processing logic
type Server struct {
	config          config.Config
	databaseConnURL string
	databaseConn    dbConn
	database        queries
	httpServer      *http.Server
	currentTimeFn   func() time.Time
}

// New creates a new Server instance
func New(c config.Config) (Server, error) {
	databaseConnURL := db.GetConnectionURL(c)
	databaseConn, err := db.Connect(databaseConnURL)
	if err != nil {
		return Server{}, fmt.Errorf(
			"failed to initiate database connection at %q: %v", databaseConnURL, err,
		)
	}

	return Server{
		config:          c,
		databaseConnURL: databaseConnURL,
		databaseConn:    databaseConn,
		httpServer: &http.Server{
			Addr:         ":" + strconv.FormatUint(uint64(c.HTTPAPIPort), 10),
			ReadTimeout:  c.HTTPRequestTimeout,
			WriteTimeout: c.HTTPRequestTimeout,
		},
		currentTimeFn: time.Now,
	}, nil
}

// ConnectDatabase establishes a connection to the database
func (s Server) ConnectDatabase(ctx context.Context) error {
	pingAttempts := 0
	// TODO: Configure exponential backoff limits
	exponentialBackoff := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
	err := backoff.Retry(
		func() error {
			pingAttempts++

			if err := s.databaseConn.Ping(); err != nil {
				log.Warnf("Failed to ping database: %v", err)

				return err
			}

			return nil
		},
		exponentialBackoff,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to connect to database at %q after %d pings: %v",
			s.databaseConnURL,
			pingAttempts,
			err,
		)
	}

	s.database = db.New(s.databaseConn)

	log.Infof("Connected to DB at %q", s.databaseConnURL)

	return nil
}

// ListenAndServe starts the HTTP server and blocks waiting for Shutdown() to be
// called from another goroutine
func (s Server) ListenAndServe() {
	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Errorf("http.ListenAndServe error: %v", err)
	}
}

// Shutdown shuts down the server gracefully
func (s Server) Shutdown(ctx context.Context) error {
	err := s.httpServer.Shutdown(ctx)

	if errDBShutdown := s.databaseConn.Close(); errDBShutdown != nil {
		if err != nil {
			return fmt.Errorf(
				"HTTP server shutdown error: %v; database connection close error: %v",
				err, errDBShutdown,
			)
		}

		return fmt.Errorf("database connection close error: %v", errDBShutdown)
	}

	if err != nil {
		return fmt.Errorf("HTTP server shutdown error: %v", err)
	}

	return nil
}
