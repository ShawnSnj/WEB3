package storage

import (
	"database/sql"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() error {
	var err error
	DB, err = sql.Open("postgres", os.Getenv("DB_URL"))
	return err
}
