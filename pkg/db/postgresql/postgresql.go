package postgresql

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	envVarDatabaseURL = "DATABASE_URL"
)

var errEnvVarNotFound = errors.New("not found environment variable")

// PostgreSQL abstracts PostgreSQL client.
type PostgreSQL struct {
	conn *pgxpool.Pool
}

// NewPostgreSQL creates a new instance of PostgreSQL object.
func NewPostgreSQL() (*PostgreSQL, error) {
	databaseURL, found := os.LookupEnv(envVarDatabaseURL)
	if !found {
		return nil, fmt.Errorf("%w: %s", errEnvVarNotFound, envVarDatabaseURL)
	}

	dbConnectionPool, err := pgxpool.Connect(context.Background(), databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to db: %w", err)
	}

	return &PostgreSQL{conn: dbConnectionPool}, nil
}

// Stop stops PostgreSQL and closes the connection pool.
func (p *PostgreSQL) Stop() {
	p.conn.Close()
}
