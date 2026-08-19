package store

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/undeschimb/undeschimb/internal/domain"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) Migrate(ctx context.Context, migrationPath string) error {
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	for _, statement := range strings.Split(string(migration), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := s.pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("run migration: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) SaveCollection(ctx context.Context, provider string, snapshots []domain.RateSnapshot, startedAt, completedAt time.Time, collectionErr error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	status := "success"
	message := ""
	if collectionErr != nil {
		status = "failed"
		message = collectionErr.Error()
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO collection_runs (provider, status, message, started_at, completed_at) VALUES ($1, $2, $3, $4, $5)`,
		provider, status, message, startedAt, completedAt); err != nil {
		return fmt.Errorf("save collection run: %w", err)
	}
	if collectionErr == nil {
		for _, snapshot := range snapshots {
			if err := snapshot.Validate(); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `
				INSERT INTO rate_snapshots (provider, currency, buy_rate, sell_rate, fee_percent, source_url, effective_at, fetched_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
				snapshot.Provider, snapshot.Currency, snapshot.BuyRate, snapshot.SellRate, snapshot.FeePercent,
				snapshot.SourceURL, snapshot.EffectiveAt, snapshot.FetchedAt); err != nil {
				return fmt.Errorf("save rate snapshot: %w", err)
			}
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) LatestRates(ctx context.Context, currency string) ([]domain.RateSnapshot, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT provider, currency, buy_rate, sell_rate, fee_percent, source_url, effective_at, fetched_at
		FROM (
			SELECT provider, currency, buy_rate, sell_rate, fee_percent, source_url, effective_at, fetched_at,
			       ROW_NUMBER() OVER (PARTITION BY provider, currency ORDER BY fetched_at DESC, id DESC) AS rank
			FROM rate_snapshots WHERE currency = $1
		) latest WHERE rank = 1`, currency)
	if err != nil {
		return nil, fmt.Errorf("query latest rates: %w", err)
	}
	defer rows.Close()

	snapshots := make([]domain.RateSnapshot, 0)
	for rows.Next() {
		var snapshot domain.RateSnapshot
		if err := rows.Scan(&snapshot.Provider, &snapshot.Currency, &snapshot.BuyRate, &snapshot.SellRate,
			&snapshot.FeePercent, &snapshot.SourceURL, &snapshot.EffectiveAt, &snapshot.FetchedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

func (s *PostgresStore) History(ctx context.Context, provider, currency string, since time.Time) ([]domain.RateSnapshot, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT provider, currency, buy_rate, sell_rate, fee_percent, source_url, effective_at, fetched_at
		FROM rate_snapshots
		WHERE provider = $1 AND currency = $2 AND fetched_at >= $3
		ORDER BY fetched_at ASC, id ASC`, provider, currency, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	snapshots := make([]domain.RateSnapshot, 0)
	for rows.Next() {
		var snapshot domain.RateSnapshot
		if err := rows.Scan(&snapshot.Provider, &snapshot.Currency, &snapshot.BuyRate, &snapshot.SellRate,
			&snapshot.FeePercent, &snapshot.SourceURL, &snapshot.EffectiveAt, &snapshot.FetchedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

func (s *PostgresStore) Providers(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT provider FROM rate_snapshots WHERE provider <> 'bnr' ORDER BY provider`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	providers := []string{}
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers, rows.Err()
}

var _ interface {
	LatestRates(context.Context, string) ([]domain.RateSnapshot, error)
	History(context.Context, string, string, time.Time) ([]domain.RateSnapshot, error)
} = (*PostgresStore)(nil)
