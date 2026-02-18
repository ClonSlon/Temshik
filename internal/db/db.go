package db

import (
	"database/sql"

	"github.com/bludnic/temchik/internal/appdata"
	_ "modernc.org/sqlite"
)

func Open() (*sql.DB, error) {
	dsn, err := appdata.DBURL()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Keep behavior close to Prisma (enforce FKs).
	_, _ = db.Exec(`PRAGMA foreign_keys=ON;`)

	return db, nil
}
