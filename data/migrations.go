package data

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

func RunMigrations(db *sql.DB, fs embed.FS) error {
	goose.SetBaseFS(fs)

	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	if err := goose.Up(db, "data/migrations"); err != nil {
		panic(err)
	}

	return nil
}
