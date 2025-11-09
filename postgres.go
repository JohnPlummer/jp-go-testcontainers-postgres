// Package postgres provides PostgreSQL testcontainer utilities for Go tests.
//
// This package offers utilities for starting PostgreSQL containers in tests,
// with support for PostGIS, automatic migration detection and running,
// and cleanup helpers for test isolation.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Error types for better error handling
var (
	ErrDockerNotAvailable    = errors.New("Docker is not available or running")
	ErrContainerStartTimeout = errors.New("container failed to start within timeout period")
	ErrContainerPortConflict = errors.New("container port conflict detected")
	ErrDatabaseConnFailed    = errors.New("failed to connect to container database")
	ErrMigrationsFailed      = errors.New("database migrations failed")
)

// DockerAvailabilityResult holds information about Docker availability
type DockerAvailabilityResult struct {
	Available bool
	Reason    string
	Error     error
}

// PostgreSQLTestContainer holds the PostgreSQL test container and related resources
type PostgreSQLTestContainer struct {
	Container    *postgres.PostgresContainer
	Pool         *pgxpool.Pool
	DatabaseURL  string
	Context      context.Context
	DatabaseName string
	Username     string
	Password     string
}

// PostgreSQLConfig provides configuration options for the PostgreSQL test container
type PostgreSQLConfig struct {
	// Database configuration
	DatabaseName string
	Username     string
	Password     string

	// Image configuration
	PostgreSQLVersion string // e.g., "16-3.4", "15-3.4" (version-postgis_version)

	// Connection configuration
	MaxConns    int32
	MinConns    int32
	MaxConnLife time.Duration
	MaxConnIdle time.Duration

	// Container configuration
	StartupTimeout time.Duration

	// Migration configuration
	RunMigrations  bool
	MigrationsPath string // Relative to the calling test file or absolute path
}

// DefaultPostgreSQLConfig returns a sensible default configuration
func DefaultPostgreSQLConfig() *PostgreSQLConfig {
	return &PostgreSQLConfig{
		DatabaseName:      "testdb",
		Username:          "testuser",
		Password:          "testpass",
		PostgreSQLVersion: "16-3.4", // PostGIS 3.4 with PostgreSQL 16 (ARM64 compatible)
		MaxConns:          10,
		MinConns:          2,
		MaxConnLife:       30 * time.Minute,
		MaxConnIdle:       5 * time.Minute,
		StartupTimeout:    30 * time.Second,
		RunMigrations:     false, // Disabled by default for simple setup
		MigrationsPath:    "",    // Will be auto-detected
	}
}

// CheckDockerAvailability checks if Docker is available and running
func CheckDockerAvailability() DockerAvailabilityResult {
	// Check if docker command exists
	_, err := exec.LookPath("docker")
	if err != nil {
		return DockerAvailabilityResult{
			Available: false,
			Reason:    "Docker command not found in PATH",
			Error:     err,
		}
	}

	// Check if Docker daemon is running
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return DockerAvailabilityResult{
			Available: false,
			Reason:    "Docker daemon is not running or accessible",
			Error:     err,
		}
	}

	// Check if we can pull images (basic functionality test)
	cmd = exec.Command("docker", "images", "--format", "table")
	if err := cmd.Run(); err != nil {
		return DockerAvailabilityResult{
			Available: false,
			Reason:    "Docker is running but images command failed",
			Error:     err,
		}
	}

	return DockerAvailabilityResult{
		Available: true,
		Reason:    "Docker is available and running",
		Error:     nil,
	}
}

// StartPostgreSQLContainerWithCheck creates and starts a PostgreSQL test container with Docker availability checks
func StartPostgreSQLContainerWithCheck(ctx context.Context, config *PostgreSQLConfig) (*PostgreSQLTestContainer, error) {
	// Check Docker availability first
	dockerStatus := CheckDockerAvailability()
	if !dockerStatus.Available {
		return nil, fmt.Errorf("%w: %s", ErrDockerNotAvailable, dockerStatus.Reason)
	}

	return StartPostgreSQLContainer(ctx, config)
}

// StartPostgreSQLContainer creates and starts a PostgreSQL test container
func StartPostgreSQLContainer(ctx context.Context, config *PostgreSQLConfig) (*PostgreSQLTestContainer, error) {
	if config == nil {
		config = DefaultPostgreSQLConfig()
	}

	// Start PostgreSQL container with enhanced error handling
	// Use PostGIS image for spatial queries (ST_DWithin, ST_MakePoint, etc.)
	pgContainer, err := postgres.Run(ctx,
		fmt.Sprintf("postgis/postgis:%s", config.PostgreSQLVersion),
		postgres.WithDatabase(config.DatabaseName),
		postgres.WithUsername(config.Username),
		postgres.WithPassword(config.Password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(config.StartupTimeout),
		),
	)
	if err != nil {
		// Enhanced error handling with specific error types
		if strings.Contains(err.Error(), "timeout") {
			return nil, fmt.Errorf("%w: %v", ErrContainerStartTimeout, err)
		}
		if strings.Contains(err.Error(), "port") && strings.Contains(err.Error(), "already in use") {
			return nil, fmt.Errorf("%w: %v", ErrContainerPortConflict, err)
		}
		return nil, fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	// Get connection details
	host, err := pgContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	// Build database URL
	databaseURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.Username, config.Password, host, port.Port(), config.DatabaseName)

	// Run migrations if requested
	if config.RunMigrations {
		if err := runMigrations(databaseURL, config.MigrationsPath); err != nil {
			_ = pgContainer.Terminate(ctx) // Cleanup on error
			return nil, fmt.Errorf("%w: %v", ErrMigrationsFailed, err)
		}
	}

	// Create connection pool
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		_ = pgContainer.Terminate(ctx) // Cleanup on error
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Apply connection pool configuration
	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnLifetime = config.MaxConnLife
	poolConfig.MaxConnIdleTime = config.MaxConnIdle

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		_ = pgContainer.Terminate(ctx) // Cleanup on error
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection with enhanced error handling
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_ = pgContainer.Terminate(ctx) // Cleanup on error
		return nil, fmt.Errorf("%w: %v", ErrDatabaseConnFailed, err)
	}

	return &PostgreSQLTestContainer{
		Container:    pgContainer,
		Pool:         pool,
		DatabaseURL:  databaseURL,
		Context:      ctx,
		DatabaseName: config.DatabaseName,
		Username:     config.Username,
		Password:     config.Password,
	}, nil
}

// StartSimplePostgreSQLContainer creates a PostgreSQL container with default settings
// This is a convenience function for simple test setups with Docker availability check
func StartSimplePostgreSQLContainer(ctx context.Context) (*PostgreSQLTestContainer, error) {
	return StartPostgreSQLContainerWithCheck(ctx, DefaultPostgreSQLConfig())
}

// StartPostgreSQLContainerWithMigrations creates a PostgreSQL container and runs migrations with Docker check
func StartPostgreSQLContainerWithMigrations(ctx context.Context, migrationsPath string) (*PostgreSQLTestContainer, error) {
	config := DefaultPostgreSQLConfig()
	config.RunMigrations = true
	config.MigrationsPath = migrationsPath
	return StartPostgreSQLContainerWithCheck(ctx, config)
}

// Close closes the connection pool and terminates the container
func (tc *PostgreSQLTestContainer) Close() error {
	var errs []error

	if tc.Pool != nil {
		tc.Pool.Close()
	}

	if tc.Container != nil {
		if err := tc.Container.Terminate(tc.Context); err != nil {
			errs = append(errs, fmt.Errorf("failed to terminate container: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}

// CleanAllTables truncates all tables in the database for test isolation
// WARNING: This removes ALL data from ALL tables
func (tc *PostgreSQLTestContainer) CleanAllTables(ctx context.Context) error {
	// Get all table names, excluding system tables
	rows, err := tc.Pool.Query(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		AND tablename NOT IN (
			'schema_migrations',
			'spatial_ref_sys',
			'geometry_columns',
			'geography_columns'
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to get table names: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating over table names: %w", err)
	}

	// Truncate all tables
	if len(tables) > 0 {
		truncateSQL := "TRUNCATE " + tables[0]
		for _, table := range tables[1:] {
			truncateSQL += ", " + table
		}
		truncateSQL += " CASCADE"

		if _, err := tc.Pool.Exec(ctx, truncateSQL); err != nil {
			return fmt.Errorf("failed to truncate tables: %w", err)
		}
	}

	return nil
}

// CleanSpecificTables truncates specific tables for test isolation
// Only truncates tables that actually exist to avoid errors
func (tc *PostgreSQLTestContainer) CleanSpecificTables(ctx context.Context, tableNames ...string) error {
	if len(tableNames) == 0 {
		return nil
	}

	// Check which tables actually exist
	var existingTables []string
	for _, tableName := range tableNames {
		var exists bool
		err := tc.Pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)",
			tableName).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check if table %s exists: %w", tableName, err)
		}
		if exists {
			existingTables = append(existingTables, tableName)
		}
	}

	// Only truncate if we have existing tables
	if len(existingTables) > 0 {
		truncateSQL := "TRUNCATE " + existingTables[0]
		for _, table := range existingTables[1:] {
			truncateSQL += ", " + table
		}
		truncateSQL += " CASCADE"

		if _, err := tc.Pool.Exec(ctx, truncateSQL); err != nil {
			return fmt.Errorf("failed to truncate specific tables: %w", err)
		}
	}

	return nil
}

// GetConnectionString returns the database connection string
func (tc *PostgreSQLTestContainer) GetConnectionString() string {
	return tc.DatabaseURL
}

// GetPool returns the connection pool
func (tc *PostgreSQLTestContainer) GetPool() *pgxpool.Pool {
	return tc.Pool
}

// GetContainer returns the testcontainers instance
func (tc *PostgreSQLTestContainer) GetContainer() *postgres.PostgresContainer {
	return tc.Container
}

// runMigrations applies database migrations
func runMigrations(databaseURL, migrationsPath string) error {
	// Auto-detect migrations path if not provided
	if migrationsPath == "" {
		migrationsPath = FindMigrationsPath()
	}

	// Convert to absolute path if relative
	if !filepath.IsAbs(migrationsPath) {
		absPath, err := filepath.Abs(migrationsPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for migrations: %w", err)
		}
		migrationsPath = absPath
	}

	// Create migrate instance
	m, err := migrate.New(
		fmt.Sprintf("file://%s?x-migrations-table=schema_migrations", migrationsPath),
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() {
		sourceErr, databaseErr := m.Close()
		if sourceErr != nil {
			fmt.Printf("Warning: failed to close migrate source: %v\n", sourceErr)
		}
		if databaseErr != nil {
			fmt.Printf("Warning: failed to close migrate database: %v\n", databaseErr)
		}
	}()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// FindMigrationsPath attempts to find the migrations directory
// This looks for common migration paths relative to the project root.
//
// First checks for MIGRATIONS_PATH environment variable, then falls back to
// walking up the directory tree from the caller's location to find the project
// root (marked by .git directory), then searches for common migration paths:
// database/migrations, migrations, sql/migrations, db/migrations.
//
// Uses .git instead of go.mod for monorepo awareness (multiple go.mod files exist).
//
// Returns "database/migrations" as fallback if no migrations directory found.
func FindMigrationsPath() string {
	// Check environment variable first for explicit override
	if envPath := os.Getenv("MIGRATIONS_PATH"); envPath != "" {
		absPath, err := filepath.Abs(envPath)
		if err == nil {
			if info, statErr := os.Stat(absPath); statErr == nil && info.IsDir() {
				return absPath
			}
		}
		// If MIGRATIONS_PATH is set but invalid, log warning but continue with fallback
		fmt.Printf("Warning: MIGRATIONS_PATH set but invalid: %s\n", envPath)
	}

	// Get the caller's file path
	_, filename, _, _ := runtime.Caller(2) // Skip current and calling function

	// Common paths to check relative to the project root
	paths := []string{
		"database/migrations",
		"migrations",
		"sql/migrations",
		"db/migrations",
	}

	// Walk up directory tree to find project root (indicated by .git)
	// We use .git instead of go.mod because this is a monorepo with multiple go.mod files
	dir := filepath.Dir(filename)
	for {
		// Check if this is project root (has .git directory)
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			// Found project root, try migration paths
			for _, path := range paths {
				fullPath := filepath.Join(dir, path)
				if pathInfo, pathErr := os.Stat(fullPath); pathErr == nil && pathInfo.IsDir() {
					return fullPath
				}
			}
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached filesystem root
		}
		dir = parent
	}

	// Fallback: assume database/migrations from current directory
	return "database/migrations"
}

// Helper functions for common test patterns

// SkipIfDockerUnavailable checks Docker availability and returns a skip message if unavailable
// This is useful for test suites that should gracefully skip when Docker is not available
func SkipIfDockerUnavailable() (shouldSkip bool, skipMessage string) {
	dockerStatus := CheckDockerAvailability()
	if !dockerStatus.Available {
		return true, fmt.Sprintf("Skipping tests: Docker not available - %s", dockerStatus.Reason)
	}
	return false, ""
}

// WithCleanup returns a cleanup function that can be deferred
func (tc *PostgreSQLTestContainer) WithCleanup() func() {
	return func() {
		if err := tc.Close(); err != nil {
			fmt.Printf("Warning: failed to cleanup PostgreSQL container: %v\n", err)
		}
	}
}

// WithTableCleanup returns a cleanup function that truncates specific tables
func (tc *PostgreSQLTestContainer) WithTableCleanup(tables ...string) func() {
	return func() {
		if err := tc.CleanSpecificTables(tc.Context, tables...); err != nil {
			fmt.Printf("Warning: failed to clean tables: %v\n", err)
		}
	}
}

// NewTestDatabase creates a new database within the container for isolation
// This is useful when you need multiple isolated databases in the same container
func (tc *PostgreSQLTestContainer) NewTestDatabase(dbName string) (string, error) {
	// Create the new database
	createSQL := fmt.Sprintf("CREATE DATABASE %s", dbName)
	if _, err := tc.Pool.Exec(tc.Context, createSQL); err != nil {
		return "", fmt.Errorf("failed to create test database %s: %w", dbName, err)
	}

	// Build connection string for the new database
	host, err := tc.Container.Host(tc.Context)
	if err != nil {
		return "", fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := tc.Container.MappedPort(tc.Context, "5432")
	if err != nil {
		return "", fmt.Errorf("failed to get container port: %w", err)
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		tc.Username, tc.Password, host, port.Port(), dbName), nil
}
