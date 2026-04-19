package storage

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// Init opens Postgres from DB_URL (e.g. in .env) and verifies the server accepts connections.
func Init() error {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		return fmt.Errorf("DB_URL is empty: set it in .env (e.g. postgresql://user:pass@localhost:5432/dbname?sslmode=disable)")
	}
	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("sql.Open: %w", err)
	}
	if err := DB.Ping(); err != nil {
		_ = DB.Close()
		DB = nil
		return fmt.Errorf("database ping failed (check DB_URL, host, port, db name, user/password): %w", err)
	}
	return nil
}
