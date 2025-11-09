package main

import (
	"context"
	"fmt"
	"log"

	postgres "github.com/JohnPlummer/go-testcontainers-postgres"
)

func main() {
	ctx := context.Background()

	// Start a PostgreSQL container
	fmt.Println("Starting PostgreSQL container...")
	tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	// Create test tables
	fmt.Println("\nCreating test tables...")
	_, err = tc.Pool.Exec(ctx, `
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}

	_, err = tc.Pool.Exec(ctx, `
		CREATE TABLE posts (
			id SERIAL PRIMARY KEY,
			user_id INTEGER,
			title TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create posts table: %v", err)
	}

	// Insert test data
	fmt.Println("Inserting test data...")
	_, err = tc.Pool.Exec(ctx, `
		INSERT INTO users (name) VALUES ('Alice'), ('Bob'), ('Charlie')
	`)
	if err != nil {
		log.Fatalf("Failed to insert users: %v", err)
	}

	_, err = tc.Pool.Exec(ctx, `
		INSERT INTO posts (user_id, title) VALUES
		(1, 'Post 1'),
		(1, 'Post 2'),
		(2, 'Post 3')
	`)
	if err != nil {
		log.Fatalf("Failed to insert posts: %v", err)
	}

	// Count data
	var userCount, postCount int
	tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM posts").Scan(&postCount)
	fmt.Printf("Users: %d, Posts: %d\n", userCount, postCount)

	// Demonstrate CleanSpecificTables
	fmt.Println("\n--- Demonstrating CleanSpecificTables ---")
	fmt.Println("Cleaning only 'posts' table...")
	if err := tc.CleanSpecificTables(ctx, "posts"); err != nil {
		log.Fatalf("Failed to clean specific tables: %v", err)
	}

	tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM posts").Scan(&postCount)
	fmt.Printf("After cleaning posts: Users: %d, Posts: %d\n", userCount, postCount)

	// Re-insert data for CleanAllTables demo
	fmt.Println("\nRe-inserting data for next demo...")
	_, err = tc.Pool.Exec(ctx, `
		INSERT INTO posts (user_id, title) VALUES
		(1, 'Post 1'),
		(1, 'Post 2'),
		(2, 'Post 3')
	`)
	if err != nil {
		log.Fatalf("Failed to re-insert posts: %v", err)
	}

	tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM posts").Scan(&postCount)
	fmt.Printf("Before cleanup: Users: %d, Posts: %d\n", userCount, postCount)

	// Demonstrate CleanAllTables
	fmt.Println("\n--- Demonstrating CleanAllTables ---")
	fmt.Println("Cleaning all tables...")
	if err := tc.CleanAllTables(ctx); err != nil {
		log.Fatalf("Failed to clean all tables: %v", err)
	}

	tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM posts").Scan(&postCount)
	fmt.Printf("After CleanAllTables: Users: %d, Posts: %d\n", userCount, postCount)

	// Demonstrate WithTableCleanup helper
	fmt.Println("\n--- Demonstrating WithTableCleanup Helper ---")
	fmt.Println("Re-inserting data...")
	_, err = tc.Pool.Exec(ctx, `
		INSERT INTO users (name) VALUES ('Dave')
	`)
	if err != nil {
		log.Fatalf("Failed to insert user: %v", err)
	}

	tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	fmt.Printf("Before deferred cleanup: Users: %d\n", userCount)

	// Setup deferred cleanup
	cleanup := tc.WithTableCleanup("users")
	defer cleanup()

	fmt.Println("Cleanup will be called when function exits...")
	fmt.Println("\nCleanup example completed successfully!")
}
