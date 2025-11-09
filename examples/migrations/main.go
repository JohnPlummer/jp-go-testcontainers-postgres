package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	postgres "github.com/JohnPlummer/go-testcontainers-postgres"
)

func main() {
	ctx := context.Background()

	// Create temporary directory for migrations
	tmpDir, err := os.MkdirTemp("", "migrations-example-*")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		log.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create migration files
	fmt.Println("Creating migration files...")
	migration1Up := `
CREATE TABLE users (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	email TEXT UNIQUE NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
`
	if err := os.WriteFile(filepath.Join(migrationsDir, "001_create_users.up.sql"), []byte(migration1Up), 0o644); err != nil {
		log.Fatalf("Failed to write migration file: %v", err)
	}

	migration1Down := "DROP TABLE IF EXISTS users CASCADE;"
	if err := os.WriteFile(filepath.Join(migrationsDir, "001_create_users.down.sql"), []byte(migration1Down), 0o644); err != nil {
		log.Fatalf("Failed to write migration file: %v", err)
	}

	migration2Up := `
CREATE TABLE posts (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	content TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_posts_user_id ON posts(user_id);
`
	if err := os.WriteFile(filepath.Join(migrationsDir, "002_create_posts.up.sql"), []byte(migration2Up), 0o644); err != nil {
		log.Fatalf("Failed to write migration file: %v", err)
	}

	migration2Down := "DROP TABLE IF EXISTS posts CASCADE;"
	if err := os.WriteFile(filepath.Join(migrationsDir, "002_create_posts.down.sql"), []byte(migration2Down), 0o644); err != nil {
		log.Fatalf("Failed to write migration file: %v", err)
	}

	fmt.Printf("Created migrations in: %s\n", migrationsDir)

	// Start PostgreSQL container with migrations
	fmt.Println("\nStarting PostgreSQL container with migrations...")
	tc, err := postgres.StartPostgreSQLContainerWithMigrations(ctx, migrationsDir)
	if err != nil {
		log.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	fmt.Println("PostgreSQL container started and migrations applied!")

	// Verify tables exist
	fmt.Println("\nVerifying tables exist...")
	var usersExists, postsExists bool
	err = tc.Pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'users')
	`).Scan(&usersExists)
	if err != nil {
		log.Fatalf("Failed to check users table: %v", err)
	}

	err = tc.Pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'posts')
	`).Scan(&postsExists)
	if err != nil {
		log.Fatalf("Failed to check posts table: %v", err)
	}

	fmt.Printf("Users table exists: %v\n", usersExists)
	fmt.Printf("Posts table exists: %v\n", postsExists)

	// Insert test data
	fmt.Println("\nInserting test data...")
	var userID int
	err = tc.Pool.QueryRow(ctx, `
		INSERT INTO users (name, email)
		VALUES ('Alice', 'alice@example.com')
		RETURNING id
	`).Scan(&userID)
	if err != nil {
		log.Fatalf("Failed to insert user: %v", err)
	}

	_, err = tc.Pool.Exec(ctx, `
		INSERT INTO posts (user_id, title, content)
		VALUES ($1, 'First Post', 'This is my first post!')
	`, userID)
	if err != nil {
		log.Fatalf("Failed to insert post: %v", err)
	}

	// Query with join
	fmt.Println("\nQuerying data with join...")
	var userName, postTitle string
	err = tc.Pool.QueryRow(ctx, `
		SELECT u.name, p.title
		FROM users u
		JOIN posts p ON u.id = p.user_id
		WHERE u.id = $1
	`, userID).Scan(&userName, &postTitle)
	if err != nil {
		log.Fatalf("Failed to query joined data: %v", err)
	}

	fmt.Printf("User: %s, Post: %s\n", userName, postTitle)

	fmt.Println("\nMigrations example completed successfully!")
}
