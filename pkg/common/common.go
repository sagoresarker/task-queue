package common

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func ConnectToDatabase(ctx context.Context, dbConnectionString string) (*pgxpool.Pool, error) {
	var dbPool *pgxpool.Pool

	var err error

	retryCount := 0

	for retryCount < 5 {
		dbPool, err = pgxpool.Connect(ctx, dbConnectionString)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to database. Retrying in 5 sec...")
		time.Sleep(5 * time.Second)
		retryCount++
	}

	if err != nil {
		log.Printf("Failed to connect to database after 5 retries")
		return nil, err
	}
	log.Printf("Connected to database")
	return dbPool, nil
}
