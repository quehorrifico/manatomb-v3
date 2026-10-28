package db

import (
	"database/sql"
	"log"
	"time"

	"manatomb/app/internal/perf"
)

func Open(databaseURL string) *sql.DB {
	connector, err := perf.PostgresConnector(databaseURL)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	db := sql.OpenDB(connector)

	// Conservative defaults for small managed Postgres plans.
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}
	return db
}
