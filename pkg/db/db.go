package db

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver for database/sql
	log "github.com/sirupsen/logrus"
)

// DB holds the database connection pool.
// We're making this a global variable so other parts of your application
// can easily access the database connection.
var DB *sqlx.DB

// InitDB initializes the database connection pool.
// It takes your database URL (which will come from your .env file)
// and sets up the connection to Neon.tech's PostgreSQL.
func InitDB(dbURL string) error {
	var err error
	// Use sqlx.Connect to establish a new database connection pool.
	// "postgres" is the driver name for PostgreSQL databases.
	// dbURL contains all the connection details, including host, user, password, and sslmode.
	DB, err = sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Errorf("Failed to connect to database: %v", err)
		return err
	}

	// Ping the database to ensure the connection is actually active and reachable.
	// This helps catch connectivity issues early during application startup.
	if err = DB.Ping(); err != nil {
		log.Errorf("Failed to ping database: %v", err)
		// Close the connection if the ping fails to clean up resources.
		DB.Close()
		return err
	}

	// SetMaxOpenConns limits the total number of active connections that can be open at once.
	// This helps prevent overloading your database (especially on a managed service like Neon.tech).
	// 25 is a common starting point, but you might adjust it based on your Neon.tech plan
	// and your application's load.
	DB.SetMaxOpenConns(25)

	// SetMaxIdleConns determines how many unused connections are kept alive in the pool.
	// These idle connections are ready for immediate reuse, reducing latency for new requests.
	// 10 is a reasonable default.
	DB.SetMaxIdleConns(10)

	// You can also set connection lifetime and idle timeout here.
	// For example, to close connections that have been idle for more than 5 minutes:
	// DB.SetConnMaxIdleTime(5 * time.Minute)
	// And to close connections after a certain total lifetime:
	// DB.SetConnMaxLifetime(5 * time.Minute)


	log.Info("Database connection pool initialized successfully.")
	return nil
}

// CloseDB closes the database connection pool.
// It's crucial to call this when your application shuts down to release resources.
// You've correctly deferred this in your main.go.
func CloseDB() {
	if DB != nil {
		if err := DB.Close(); err != nil {
			log.Errorf("Error closing database connection: %v", err)
		} else {
			log.Info("Database connection pool closed.")
		}
	}
}