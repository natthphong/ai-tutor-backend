package storage

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
)

//go:embed schema.sql
var Schema string

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	c, e := pgxpool.ParseConfig(url)
	if e != nil {
		return nil, e
	}
	if !strings.Contains(strings.ToLower(c.ConnConfig.Database), "toko") {
		return nil, fmt.Errorf("new database name must contain toko; refusing legacy database")
	}
	c.MaxConns = 12
	p, e := pgxpool.NewWithConfig(ctx, c)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, e
	}
	return p, nil
}
func Migrate(ctx context.Context, p *pgxpool.Pool) error {
	tx, e := p.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7214902)"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, Schema); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
