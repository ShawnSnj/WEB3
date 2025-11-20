package storage

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func InitMySQL(dsn string) *sql.DB {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("MySQL connect error: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("MySQL ping error: %v", err)
	}
	log.Println("✅ Connected to MySQL")
	return db
}
