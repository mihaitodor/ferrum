package db

import (
	"database/sql"
	"fmt"

	// Import postgres driver
	_ "github.com/lib/pq"
	"github.com/mihaitodor/ferrum/config"
)

// GetConnectionURL constructs the Postgres connection URL
func GetConnectionURL(c config.Config) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.DatabaseUser,
		c.DatabasePassword,
		c.DatabaseHost,
		c.DatabasePort,
		c.DatabaseName,
	)
}

// Connect initiates a connection to a Postgres database
func Connect(connURL string) (*sql.DB, error) {
	conn, err := sql.Open("postgres", connURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database at %q: %v", connURL, err)
	}

	return conn, nil
}
