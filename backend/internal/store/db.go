// Package store handles PostgreSQL connectivity and schema migrations.
package store

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a *pgxpool.Pool and is the main handle for all database operations.
type DB struct {
	Pool *pgxpool.Pool
}

// Open creates and validates a DB using the provided DSN and connection limits.
func Open(ctx context.Context, dsn string, maxConns, minConns int32) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = minConns

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Ping delegates to the underlying pool and satisfies handlers.Pinger.
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// Close releases all connections in the pool.
func (db *DB) Close() {
	db.Pool.Close()
}

// prefixTSQuery converts a raw user query string into a PostgreSQL tsquery
// that matches each word as a prefix, enabling live-search-as-you-type.
// For example "test ser" becomes "test:* & ser:*".
//
// tsquery special characters (! | & ( ) : ') are stripped before splitting
// so the input is safe to pass directly to to_tsquery().
func prefixTSQuery(raw string) string {
	// Strip tsquery operator characters to prevent injection / syntax errors.
	clean := strings.Map(func(r rune) rune {
		switch r {
		case '!', '|', '&', '(', ')', ':', '\'', '<', '>':
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, raw)

	var terms []string
	for _, word := range strings.Fields(clean) {
		if word != "" {
			terms = append(terms, word+":*")
		}
	}
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " & ")
}
