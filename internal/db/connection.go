package db

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// DB wraps a pgx connection with metadata.
type DB struct {
	Conn       *pgx.Conn
	connString string
	host       string
	port       string
	user       string
	database   string
}

// Connect establishes a PostgreSQL connection with a 10-second timeout.
func Connect(host, port, user, password, database string) (*DB, error) {
	encodedPassword := url.QueryEscape(password)
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=prefer",
		user, encodedPassword, host, port, database)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return nil, err
	}

	return &DB{
		Conn:       conn,
		connString: connStr,
		host:       host,
		port:       port,
		user:       user,
		database:   database,
	}, nil
}

// ConnectURI establishes a PostgreSQL connection from a raw URI string.
func ConnectURI(uri string) (*DB, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	user := parsed.User.Username()
	database := strings.TrimPrefix(parsed.Path, "/")

	// Ensure sslmode is set if not already present
	q := parsed.Query()
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "prefer")
		parsed.RawQuery = q.Encode()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, parsed.String())
	if err != nil {
		return nil, err
	}

	return &DB{
		Conn:       conn,
		connString: parsed.String(),
		host:       host,
		port:       port,
		user:       user,
		database:   database,
	}, nil
}

// Close closes the database connection.
func (d *DB) Close() {
	if d.Conn != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		d.Conn.Close(ctx)
	}
}

// IsConnected checks if the connection is alive.
func (d *DB) IsConnected() bool {
	if d.Conn == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return d.Conn.Ping(ctx) == nil
}

// ConnInfo returns a display-safe connection string (no password).
func (d *DB) ConnInfo() string {
	return fmt.Sprintf("postgres://%s@%s:%s/%s", d.user, d.host, d.port, d.database)
}
