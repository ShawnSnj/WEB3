package storage

import (
	"database/sql"
     "os"
	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() error {
	var err error
	DB, err = sql.Open("postgres", os.Getenv("DB_URL"))
	return err
}
