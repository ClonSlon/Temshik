package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"time"
)

func ApplyPrismaMigrations(db *sql.DB, migrations fs.FS) error {
	if err := ensurePrismaMigrationsTable(db); err != nil {
		return err
	}

	applied, err := getAppliedMigrations(db)
	if err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return err
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		if applied[dir] {
			continue
		}

		sqlBytes, err := fs.ReadFile(migrations, dir+"/migration.sql")
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}

		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", dir, err)
		}

		id, err := randomHex(16)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		checksum := sha256.Sum256(sqlBytes)
		now := time.Now().UTC()

		if _, err := tx.Exec(
			`INSERT INTO "_prisma_migrations" (id, checksum, finished_at, migration_name, started_at, applied_steps_count) VALUES (?, ?, ?, ?, ?, ?)`,
			id,
			hex.EncodeToString(checksum[:]),
			now,
			dir,
			now,
			1,
		); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func ensurePrismaMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS "_prisma_migrations" (
  "id" TEXT NOT NULL PRIMARY KEY,
  "checksum" TEXT NOT NULL,
  "finished_at" DATETIME,
  "migration_name" TEXT NOT NULL,
  "logs" TEXT,
  "rolled_back_at" DATETIME,
  "started_at" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "applied_steps_count" INTEGER NOT NULL DEFAULT 0
);`)
	return err
}

func getAppliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT migration_name FROM "_prisma_migrations" WHERE rolled_back_at IS NULL AND finished_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return applied, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
