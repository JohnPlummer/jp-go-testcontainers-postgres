//go:build integration

package postgres

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartSimplePostgreSQLContainer(t *testing.T) {
	ctx := context.Background()

	tc, err := StartSimplePostgreSQLContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	// Verify connection
	if err := tc.Pool.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Verify connection string
	if tc.GetConnectionString() == "" {
		t.Error("Expected non-empty connection string")
	}

	// Verify pool
	if tc.GetPool() == nil {
		t.Error("Expected non-nil pool")
	}

	// Verify container
	if tc.GetContainer() == nil {
		t.Error("Expected non-nil container")
	}
}

func TestStartPostgreSQLContainerWithCustomConfig(t *testing.T) {
	ctx := context.Background()

	config := &PostgreSQLConfig{
		DatabaseName:      "customdb",
		Username:          "customuser",
		Password:          "custompass",
		PostgreSQLVersion: "16-3.4",
		MaxConns:          5,
		MinConns:          1,
		MaxConnLife:       10 * time.Minute,
		MaxConnIdle:       2 * time.Minute,
		StartupTimeout:    45 * time.Second,
		RunMigrations:     false,
		MigrationsPath:    "",
	}

	tc, err := StartPostgreSQLContainer(ctx, config)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	// Verify database name
	if tc.DatabaseName != "customdb" {
		t.Errorf("Expected database name to be customdb, got %s", tc.DatabaseName)
	}

	// Verify username
	if tc.Username != "customuser" {
		t.Errorf("Expected username to be customuser, got %s", tc.Username)
	}

	// Verify connection
	if err := tc.Pool.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}
}

func TestPostGISExtension(t *testing.T) {
	ctx := context.Background()

	tc, err := StartSimplePostgreSQLContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	// Check if PostGIS extension is available
	var postgisVersion string
	err = tc.Pool.QueryRow(ctx, "SELECT PostGIS_Version()").Scan(&postgisVersion)
	if err != nil {
		t.Fatalf("Failed to query PostGIS version: %v", err)
	}

	if postgisVersion == "" {
		t.Error("Expected PostGIS version to be non-empty")
	}

	t.Logf("PostGIS version: %s", postgisVersion)

	// Test basic PostGIS functionality
	_, err = tc.Pool.Exec(ctx, `
		CREATE TABLE test_locations (
			id SERIAL PRIMARY KEY,
			name TEXT,
			location GEOGRAPHY(POINT)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table with geography column: %v", err)
	}

	// Insert test data
	_, err = tc.Pool.Exec(ctx, `
		INSERT INTO test_locations (name, location)
		VALUES ('Test Point', ST_MakePoint(-122.4194, 37.7749))
	`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Query test data
	var count int
	err = tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_locations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query test data: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}
}

func TestCleanAllTables(t *testing.T) {
	ctx := context.Background()

	tc, err := StartSimplePostgreSQLContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	// Create test tables
	_, err = tc.Pool.Exec(ctx, `
		CREATE TABLE test_users (
			id SERIAL PRIMARY KEY,
			name TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test_users table: %v", err)
	}

	_, err = tc.Pool.Exec(ctx, `
		CREATE TABLE test_posts (
			id SERIAL PRIMARY KEY,
			user_id INT,
			title TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test_posts table: %v", err)
	}

	// Insert test data
	_, err = tc.Pool.Exec(ctx, "INSERT INTO test_users (name) VALUES ('Alice'), ('Bob')")
	if err != nil {
		t.Fatalf("Failed to insert test users: %v", err)
	}

	_, err = tc.Pool.Exec(ctx, "INSERT INTO test_posts (user_id, title) VALUES (1, 'Post 1'), (2, 'Post 2')")
	if err != nil {
		t.Fatalf("Failed to insert test posts: %v", err)
	}

	// Verify data exists
	var userCount int
	err = tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_users").Scan(&userCount)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}
	if userCount != 2 {
		t.Errorf("Expected 2 users, got %d", userCount)
	}

	// Clean all tables
	if err := tc.CleanAllTables(ctx); err != nil {
		t.Fatalf("Failed to clean all tables: %v", err)
	}

	// Verify data is removed
	err = tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_users").Scan(&userCount)
	if err != nil {
		t.Fatalf("Failed to count users after cleanup: %v", err)
	}
	if userCount != 0 {
		t.Errorf("Expected 0 users after cleanup, got %d", userCount)
	}

	var postCount int
	err = tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_posts").Scan(&postCount)
	if err != nil {
		t.Fatalf("Failed to count posts after cleanup: %v", err)
	}
	if postCount != 0 {
		t.Errorf("Expected 0 posts after cleanup, got %d", postCount)
	}
}

func TestCleanSpecificTables(t *testing.T) {
	ctx := context.Background()

	tc, err := StartSimplePostgreSQLContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	// Create test tables
	_, err = tc.Pool.Exec(ctx, "CREATE TABLE test_table1 (id SERIAL PRIMARY KEY, data TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test_table1: %v", err)
	}

	_, err = tc.Pool.Exec(ctx, "CREATE TABLE test_table2 (id SERIAL PRIMARY KEY, data TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test_table2: %v", err)
	}

	// Insert test data
	_, err = tc.Pool.Exec(ctx, "INSERT INTO test_table1 (data) VALUES ('data1')")
	if err != nil {
		t.Fatalf("Failed to insert into test_table1: %v", err)
	}

	_, err = tc.Pool.Exec(ctx, "INSERT INTO test_table2 (data) VALUES ('data2')")
	if err != nil {
		t.Fatalf("Failed to insert into test_table2: %v", err)
	}

	// Clean only table1
	if err := tc.CleanSpecificTables(ctx, "test_table1"); err != nil {
		t.Fatalf("Failed to clean specific tables: %v", err)
	}

	// Verify table1 is empty
	var count1 int
	err = tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_table1").Scan(&count1)
	if err != nil {
		t.Fatalf("Failed to count test_table1: %v", err)
	}
	if count1 != 0 {
		t.Errorf("Expected 0 rows in test_table1, got %d", count1)
	}

	// Verify table2 still has data
	var count2 int
	err = tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_table2").Scan(&count2)
	if err != nil {
		t.Fatalf("Failed to count test_table2: %v", err)
	}
	if count2 != 1 {
		t.Errorf("Expected 1 row in test_table2, got %d", count2)
	}
}

func TestNewTestDatabase(t *testing.T) {
	ctx := context.Background()

	tc, err := StartSimplePostgreSQLContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	// Create a new database
	newDBURL, err := tc.NewTestDatabase("test_new_db")
	if err != nil {
		t.Fatalf("Failed to create new test database: %v", err)
	}

	if newDBURL == "" {
		t.Error("Expected non-empty database URL")
	}

	t.Logf("New database URL: %s", newDBURL)

	// Verify the URL format is correct
	if newDBURL == tc.DatabaseURL {
		t.Error("New database URL should be different from original")
	}

	// Verify the new database name is in the URL
	if !strings.Contains(newDBURL, "test_new_db") {
		t.Error("New database URL should contain the database name")
	}
}

func TestStartPostgreSQLContainerWithMigrations(t *testing.T) {
	ctx := context.Background()

	// Create temporary migrations directory
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create test migration files
	migration1Up := `
CREATE TABLE users (
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	email TEXT UNIQUE NOT NULL
);
`
	if err := os.WriteFile(filepath.Join(migrationsDir, "001_create_users.up.sql"), []byte(migration1Up), 0o644); err != nil {
		t.Fatalf("Failed to write migration file: %v", err)
	}

	migration1Down := "DROP TABLE IF EXISTS users;"
	if err := os.WriteFile(filepath.Join(migrationsDir, "001_create_users.down.sql"), []byte(migration1Down), 0o644); err != nil {
		t.Fatalf("Failed to write migration file: %v", err)
	}

	// Start container with migrations
	tc, err := StartPostgreSQLContainerWithMigrations(ctx, migrationsDir)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container with migrations: %v", err)
	}
	defer tc.Close()

	// Verify migration was applied
	var exists bool
	err = tc.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'users'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check if users table exists: %v", err)
	}

	if !exists {
		t.Error("Expected users table to exist after migrations")
	}

	// Verify schema_migrations table exists
	err = tc.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables
			WHERE table_name = 'schema_migrations'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check if schema_migrations table exists: %v", err)
	}

	if !exists {
		t.Error("Expected schema_migrations table to exist")
	}
}

func TestWithCleanupDeferred(t *testing.T) {
	ctx := context.Background()

	tc, err := StartSimplePostgreSQLContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.WithCleanup()()

	// Verify connection works
	if err := tc.Pool.Ping(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Create test table
	_, err = tc.Pool.Exec(ctx, "CREATE TABLE test_cleanup (id SERIAL PRIMARY KEY)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
}

func TestWithTableCleanupDeferred(t *testing.T) {
	ctx := context.Background()

	tc, err := StartSimplePostgreSQLContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}
	defer tc.Close()

	// Create test table
	_, err = tc.Pool.Exec(ctx, "CREATE TABLE test_cleanup (id SERIAL PRIMARY KEY, data TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert data
	_, err = tc.Pool.Exec(ctx, "INSERT INTO test_cleanup (data) VALUES ('test1'), ('test2')")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Setup cleanup for the table
	cleanup := tc.WithTableCleanup("test_cleanup")
	defer cleanup()

	// Verify data exists before cleanup
	var count int
	err = tc.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_cleanup").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 rows before cleanup, got %d", count)
	}
}
