package main

import (
	"context"
	"fmt"
	"log"

	postgres "github.com/JohnPlummer/go-testcontainers-postgres"
)

func main() {
	ctx := context.Background()

	// Start a PostgreSQL container with default settings
	fmt.Println("Starting PostgreSQL container...")
	tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	fmt.Printf("PostgreSQL container started successfully\n")
	fmt.Printf("Database URL: %s\n", tc.GetConnectionString())

	// Create a test table
	fmt.Println("\nCreating test table...")
	_, err = tc.Pool.Exec(ctx, `
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data
	fmt.Println("Inserting test data...")
	_, err = tc.Pool.Exec(ctx, `
		INSERT INTO users (name, email) VALUES
		('Alice', 'alice@example.com'),
		('Bob', 'bob@example.com'),
		('Charlie', 'charlie@example.com')
	`)
	if err != nil {
		log.Fatalf("Failed to insert data: %v", err)
	}

	// Query the data
	fmt.Println("\nQuerying data...")
	rows, err := tc.Pool.Query(ctx, "SELECT id, name, email FROM users ORDER BY id")
	if err != nil {
		log.Fatalf("Failed to query data: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, email string
		if err := rows.Scan(&id, &name, &email); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		fmt.Printf("ID: %d, Name: %s, Email: %s\n", id, name, email)
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating rows: %v", err)
	}

	fmt.Println("\nExample completed successfully!")
}
