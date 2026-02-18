package db

import "database/sql"

func Seed(db *sql.DB) error {
	_, err := db.Exec(
		`INSERT INTO "User" ("id", "email", "displayName", "role")
VALUES (1, ?, ?, ?)
ON CONFLICT("id") DO UPDATE SET
  "email"=excluded."email",
  "displayName"=excluded."displayName",
  "role"=excluded."role";`,
		"onboarding@temchik.pro",
		"Temchik",
		"Admin",
	)
	return err
}
