package postgres

import (
	"database/sql"
	"embed"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func RunMigrations(pool *pgxpool.Pool) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	sqlDB, err := sql.Open("pgx", pool.Config().ConnString())
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	return goose.Up(sqlDB, "migrations")
}
