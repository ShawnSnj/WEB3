package storage

import (
	"database/sql"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() error {
	var err error
	DB, err = sql.Open("postgres", "host=localhost port=5432 user=postgres password=mysecret dbname=defi_pnl sslmode=disable")
	return err
}
