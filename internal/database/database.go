// File: internal/database/database.go
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB represents the database connection pool
type DB struct {
	Pool *pgxpool.Pool
}

// New creates a new database connection pool
func New(databaseURL string) (*DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	// Create a connection pool instead of a single connection for better performance
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// GetVersion returns the database version
func (db *DB) GetVersion() (string, error) {
	var version string
	err := db.Pool.QueryRow(context.Background(), "SELECT version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to query database version: %w", err)
	}
	return version, nil
}
