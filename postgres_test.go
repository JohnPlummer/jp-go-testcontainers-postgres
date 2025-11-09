package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestDefaultPostgreSQLConfig(t *testing.T) {
	config := DefaultPostgreSQLConfig()

	if config.DatabaseName != "testdb" {
		t.Errorf("Expected DatabaseName to be testdb, got %s", config.DatabaseName)
	}
	if config.Username != "testuser" {
		t.Errorf("Expected Username to be testuser, got %s", config.Username)
	}
	if config.Password != "testpass" {
		t.Errorf("Expected Password to be testpass, got %s", config.Password)
	}
	if config.PostgreSQLVersion != "16-3.4" {
		t.Errorf("Expected PostgreSQLVersion to be 16-3.4, got %s", config.PostgreSQLVersion)
	}
	if config.MaxConns != 10 {
		t.Errorf("Expected MaxConns to be 10, got %d", config.MaxConns)
	}
	if config.MinConns != 2 {
		t.Errorf("Expected MinConns to be 2, got %d", config.MinConns)
	}
	if config.MaxConnLife != 30*time.Minute {
		t.Errorf("Expected MaxConnLife to be 30m, got %v", config.MaxConnLife)
	}
	if config.MaxConnIdle != 5*time.Minute {
		t.Errorf("Expected MaxConnIdle to be 5m, got %v", config.MaxConnIdle)
	}
	if config.StartupTimeout != 30*time.Second {
		t.Errorf("Expected StartupTimeout to be 30s, got %v", config.StartupTimeout)
	}
	if config.RunMigrations {
		t.Error("Expected RunMigrations to be false")
	}
	if config.MigrationsPath != "" {
		t.Errorf("Expected MigrationsPath to be empty, got %s", config.MigrationsPath)
	}
}

func TestPostgreSQLConfig_CustomValues(t *testing.T) {
	config := &PostgreSQLConfig{
		DatabaseName:      "customdb",
		Username:          "customuser",
		Password:          "custompass",
		PostgreSQLVersion: "15-3.3",
		MaxConns:          20,
		MinConns:          5,
		MaxConnLife:       1 * time.Hour,
		MaxConnIdle:       10 * time.Minute,
		StartupTimeout:    60 * time.Second,
		RunMigrations:     true,
		MigrationsPath:    "/custom/migrations",
	}

	if config.DatabaseName != "customdb" {
		t.Errorf("Expected DatabaseName to be customdb, got %s", config.DatabaseName)
	}
	if config.Username != "customuser" {
		t.Errorf("Expected Username to be customuser, got %s", config.Username)
	}
	if config.Password != "custompass" {
		t.Errorf("Expected Password to be custompass, got %s", config.Password)
	}
	if config.PostgreSQLVersion != "15-3.3" {
		t.Errorf("Expected PostgreSQLVersion to be 15-3.3, got %s", config.PostgreSQLVersion)
	}
	if config.MaxConns != 20 {
		t.Errorf("Expected MaxConns to be 20, got %d", config.MaxConns)
	}
	if config.MinConns != 5 {
		t.Errorf("Expected MinConns to be 5, got %d", config.MinConns)
	}
	if config.MaxConnLife != 1*time.Hour {
		t.Errorf("Expected MaxConnLife to be 1h, got %v", config.MaxConnLife)
	}
	if config.MaxConnIdle != 10*time.Minute {
		t.Errorf("Expected MaxConnIdle to be 10m, got %v", config.MaxConnIdle)
	}
	if config.StartupTimeout != 60*time.Second {
		t.Errorf("Expected StartupTimeout to be 60s, got %v", config.StartupTimeout)
	}
	if !config.RunMigrations {
		t.Error("Expected RunMigrations to be true")
	}
	if config.MigrationsPath != "/custom/migrations" {
		t.Errorf("Expected MigrationsPath to be /custom/migrations, got %s", config.MigrationsPath)
	}
}

func TestCheckDockerAvailability(t *testing.T) {
	result := CheckDockerAvailability()

	// Just ensure the function runs without panicking
	// Actual availability depends on the environment
	if result.Available {
		if result.Reason != "Docker is available and running" {
			t.Errorf("Expected reason to be 'Docker is available and running', got %s", result.Reason)
		}
		if result.Error != nil {
			t.Errorf("Expected no error when Docker is available, got %v", result.Error)
		}
	} else {
		if result.Reason == "" {
			t.Error("Expected a reason when Docker is not available")
		}
	}
}

func TestDockerAvailabilityResult_Fields(t *testing.T) {
	tests := []struct {
		name       string
		result     DockerAvailabilityResult
		wantAvail  bool
		wantReason string
		wantErr    bool
	}{
		{
			name: "available",
			result: DockerAvailabilityResult{
				Available: true,
				Reason:    "Docker is available",
				Error:     nil,
			},
			wantAvail:  true,
			wantReason: "Docker is available",
			wantErr:    false,
		},
		{
			name: "not available",
			result: DockerAvailabilityResult{
				Available: false,
				Reason:    "Docker not found",
				Error:     errors.New("command not found"),
			},
			wantAvail:  false,
			wantReason: "Docker not found",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Available != tt.wantAvail {
				t.Errorf("Available = %v, want %v", tt.result.Available, tt.wantAvail)
			}
			if tt.result.Reason != tt.wantReason {
				t.Errorf("Reason = %v, want %v", tt.result.Reason, tt.wantReason)
			}
			if (tt.result.Error != nil) != tt.wantErr {
				t.Errorf("Error = %v, wantErr %v", tt.result.Error, tt.wantErr)
			}
		})
	}
}

func TestSkipIfDockerUnavailable(t *testing.T) {
	shouldSkip, skipMessage := SkipIfDockerUnavailable()

	// Just ensure the function runs without panicking
	if shouldSkip {
		if skipMessage == "" {
			t.Error("Expected a skip message when Docker is unavailable")
		}
	} else {
		if skipMessage != "" {
			t.Errorf("Expected empty skip message when Docker is available, got %s", skipMessage)
		}
	}
}

func TestFindMigrationsPath(t *testing.T) {
	// Save original env
	origPath := os.Getenv("MIGRATIONS_PATH")
	defer func() {
		if origPath != "" {
			os.Setenv("MIGRATIONS_PATH", origPath)
		} else {
			os.Unsetenv("MIGRATIONS_PATH")
		}
	}()

	// Test with no env var (should use fallback)
	os.Unsetenv("MIGRATIONS_PATH")
	path := FindMigrationsPath()
	if path == "" {
		t.Error("Expected a non-empty path")
	}

	// Test with env var pointing to temp dir
	tmpDir := t.TempDir()
	os.Setenv("MIGRATIONS_PATH", tmpDir)
	path = FindMigrationsPath()
	if path != tmpDir {
		t.Errorf("Expected path to be %s, got %s", tmpDir, path)
	}
}

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "docker not available",
			err:  ErrDockerNotAvailable,
			want: "Docker is not available or running",
		},
		{
			name: "container start timeout",
			err:  ErrContainerStartTimeout,
			want: "container failed to start within timeout period",
		},
		{
			name: "port conflict",
			err:  ErrContainerPortConflict,
			want: "container port conflict detected",
		},
		{
			name: "database connection failed",
			err:  ErrDatabaseConnFailed,
			want: "failed to connect to container database",
		},
		{
			name: "migrations failed",
			err:  ErrMigrationsFailed,
			want: "database migrations failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("Error message = %v, want %v", tt.err.Error(), tt.want)
			}
		})
	}
}

func TestPostgreSQLTestContainer_Accessors(t *testing.T) {
	// Create a minimal container struct for testing accessors
	tc := &PostgreSQLTestContainer{
		DatabaseURL:  "postgres://test:test@localhost:5432/testdb",
		Context:      context.Background(),
		DatabaseName: "testdb",
		Username:     "testuser",
		Password:     "testpass",
	}

	if tc.GetConnectionString() != "postgres://test:test@localhost:5432/testdb" {
		t.Errorf("Expected connection string to match, got %s", tc.GetConnectionString())
	}

	if tc.GetPool() != nil {
		t.Error("Expected pool to be nil")
	}

	if tc.GetContainer() != nil {
		t.Error("Expected container to be nil")
	}
}

func TestPostgreSQLTestContainer_WithCleanup(t *testing.T) {
	tc := &PostgreSQLTestContainer{
		Context: context.Background(),
	}

	cleanup := tc.WithCleanup()
	if cleanup == nil {
		t.Error("Expected cleanup function to be non-nil")
	}

	// Call cleanup (should not panic even with nil container/pool)
	cleanup()
}

func TestPostgreSQLTestContainer_WithTableCleanup(t *testing.T) {
	tc := &PostgreSQLTestContainer{
		Context: context.Background(),
	}

	cleanup := tc.WithTableCleanup("table1", "table2")
	if cleanup == nil {
		t.Error("Expected cleanup function to be non-nil")
	}

	// Note: We can't call cleanup() here because it requires a valid pool
	// The function is tested in integration tests with real containers
}

func TestStartPostgreSQLContainer_NilConfig(t *testing.T) {
	// This test verifies that nil config uses defaults
	// We can't actually start a container in unit tests, but we can verify
	// the config logic by testing with a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediately cancel to prevent actual container start

	_, err := StartPostgreSQLContainer(ctx, nil)
	if err == nil {
		t.Error("Expected error with canceled context")
	}
	// Error should be context-related, not config-related
	if err.Error() == "config cannot be nil" {
		t.Error("Should use default config when nil is passed")
	}
}
