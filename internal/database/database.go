package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	db_sqlc "go-note/internal/db_sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload"
)

// Service represents a service that interacts with a database.
type Service interface {
	// Health returns a map of health status information.
	// The keys and values in the map are service-specific.
	Health() map[string]string

	// Close terminates the database connection.
	// It returns an error if the connection cannot be closed.
	Close() error

	// GetPool returns the database connection pool
	GetPool() *pgxpool.Pool

	// GetQueries returns the sqlc queries instance
	GetQueries() *db_sqlc.Queries
}

type service struct {
	db      *pgxpool.Pool
	queries *db_sqlc.Queries
}

var (
	database   = os.Getenv("SYMPHONY_DB_DATABASE")
	password   = os.Getenv("SYMPHONY_DB_PASSWORD")
	username   = os.Getenv("SYMPHONY_DB_USERNAME")
	port       = os.Getenv("SYMPHONY_DB_PORT")
	host       = os.Getenv("SYMPHONY_DB_HOST")
	sslmode    = os.Getenv("SYMPHONY_DB_SSLMODE")
	dbInstance *service
)

func New() Service {
	// Reuse Connection
	if dbInstance != nil {
		return dbInstance
	}
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", username, password, host, port, database, sslmode)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Fatal("Failed to parse config:", err)
	}

	config.MaxConns = 30
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = time.Minute * 30

	db, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatal("Failed to create connection pool:", err)
	}

	dbInstance = &service{
		db:      db,
		queries: db_sqlc.New(db),
	}
	return dbInstance
}

// Health checks the health of the database connection by pinging the database.
// It returns a map with keys indicating various health statistics.
func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	// Ping the database
	err := s.db.Ping(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		log.Fatalf("db down: %v", err) // Log the error and terminate the program
		return stats
	}

	// Database is up, add more statistics
	stats["status"] = "up"
	stats["message"] = "It's healthy"

	// Get database stats from pgxpool
	dbStats := s.db.Stat()
	stats["total_conns"] = strconv.Itoa(int(dbStats.TotalConns()))
	stats["acquired_conns"] = strconv.Itoa(int(dbStats.AcquiredConns()))
	stats["idle_conns"] = strconv.Itoa(int(dbStats.IdleConns()))
	stats["max_conns"] = strconv.Itoa(int(dbStats.MaxConns()))
	stats["acquire_count"] = strconv.FormatInt(dbStats.AcquireCount(), 10)
	stats["acquire_duration"] = dbStats.AcquireDuration().String()

	// Evaluate stats to provide a health message
	if dbStats.AcquiredConns() > dbStats.MaxConns()*8/10 { // More than 80% of max connections
		stats["message"] = "The database connection pool is experiencing heavy load."
	}

	if dbStats.AcquireCount() > 1000 {
		stats["message"] = "The database has a high number of connection acquisitions."
	}

	return stats
}

// Close closes the database connection pool.
// It logs a message indicating the disconnection from the specific database.
func (s *service) Close() error {
	log.Printf("Disconnected from database: %s", database)
	s.db.Close()
	return nil
}

// GetPool returns the database connection pool
func (s *service) GetPool() *pgxpool.Pool {
	return s.db
}

// GetQueries returns the sqlc queries instance
func (s *service) GetQueries() *db_sqlc.Queries {
	return s.queries
}
