package database

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/topi314/gomigrate"
	"github.com/topi314/gomigrate/drivers/sqlite"

	"github.com/topi314/campfire-event-manager/server/database/dbsqlc"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DB is a Postgres pool with sqlc queries. Call sites use sqlc methods directly
// (embedded *dbsqlc.Queries).
type DB struct {
	Pool *pgxpool.Pool
	*dbsqlc.Queries
}

func New(cfg Config) (*DB, error) {
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connectCancel()

	pool, err := pgxpool.New(connectCtx, cfg.DataSourceName())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	migrateDB := stdlib.OpenDBFromPool(pool)
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer migrateCancel()
	if err = gomigrate.Migrate(migrateCtx, migrateDB, sqlite.New, migrations); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	db := &DB{
		Pool:    pool,
		Queries: dbsqlc.New(pool),
	}
	go db.cleanup()
	return db, nil
}

func (d *DB) Close() {
	d.Pool.Close()
}

func (d *DB) cleanup() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := d.DeleteExpiredSessions(ctx); err != nil {
			slog.Error("failed to cleanup expired sessions", slog.Any("err", err))
		}
		cancel()
		time.Sleep(5 * time.Minute)
	}
}
