package postgres

import (
	"fmt"

	"github.com/TheEmfield/chat-rooms/backend/internal/config"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const POSTGRES_DRIVER string = "postgres"

type Postgres struct {
	db *sqlx.DB
}

func New(cfg *config.Config) (*Postgres, error) {
	dataSourceName := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.User, cfg.Postgres.Password,
		cfg.Postgres.DB, cfg.Postgres.SSLMode,
	)

	psql, err := sqlx.Open(POSTGRES_DRIVER, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := psql.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Postgres{psql}, nil
}

func (p *Postgres) Close() error {
	if err := p.db.Close(); err != nil {
		return fmt.Errorf("close postgres: %w", err)
	}

	return nil
}
