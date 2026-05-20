package postgres

import (
	"database/sql"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// RunMigrations применяет SQL-миграции из переданной файловой системы.
// migrationsFS — embed.FS (или любой fs.FS) с .sql файлами,
// dir — путь к папке с миграциями внутри FS (например "migrations").
func RunMigrations(pool *pgxpool.Pool, migrationsFS fs.FS, dir string) error {
	goose.SetBaseFS(migrationsFS)

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	sqlDB, err := sql.Open("pgx", pool.Config().ConnString())
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	return goose.Up(sqlDB, dir)
}
