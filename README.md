# go-testcontainers-postgres

PostgreSQL testcontainer utilities for Go tests with PostGIS support, automatic migration detection and running, and cleanup helpers for test isolation.

## Features

- **PostgreSQL with PostGIS**: Uses `postgis/postgis:16-3.4` image for geospatial queries
- **Docker availability checking**: Detailed error messages when Docker is unavailable
- **Automatic migration detection**: Auto-discovers and runs database migrations
- **Test isolation utilities**: `CleanAllTables()` and `CleanSpecificTables()` for cleanup
- **Multiple database support**: Create isolated databases within the same container
- **Connection pooling**: Configurable connection pool settings
- **Enhanced error handling**: Specific error types for common failure scenarios
- **Helper functions**: Deferred cleanup patterns for easy test setup

## Requirements

- **Docker**: Docker must be installed and running
- **Go**: 1.21 or higher

### Installing Docker

**macOS:**

```bash
brew install docker
# Or download Docker Desktop from https://www.docker.com/products/docker-desktop
```

**Linux:**

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install docker.io

# Fedora
sudo dnf install docker

# Start Docker service
sudo systemctl start docker
sudo systemctl enable docker
```

**Windows:**
Download and install Docker Desktop from <https://www.docker.com/products/docker-desktop>

## Installation

```bash
go get github.com/JohnPlummer/go-testcontainers-postgres
```

## Quick Start

```go
package mypackage_test

import (
 "context"
 "testing"

 postgres "github.com/JohnPlummer/go-testcontainers-postgres"
)

func TestMyFunction(t *testing.T) {
 ctx := context.Background()

 // Start PostgreSQL container with default settings
 tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
 if err != nil {
  t.Fatalf("Failed to start PostgreSQL container: %v", err)
 }
 defer tc.Close()

 // Use the connection pool
 _, err = tc.Pool.Exec(ctx, `
  CREATE TABLE users (
   id SERIAL PRIMARY KEY,
   name TEXT NOT NULL
  )
 `)
 if err != nil {
  t.Fatalf("Failed to create table: %v", err)
 }

 // Your test logic here
}
```

## Configuration Options

Create a custom configuration:

```go
config := &postgres.PostgreSQLConfig{
 DatabaseName:      "customdb",
 Username:          "customuser",
 Password:          "custompass",
 PostgreSQLVersion: "16-3.4", // PostgreSQL 16 with PostGIS 3.4
 MaxConns:          10,
 MinConns:          2,
 MaxConnLife:       30 * time.Minute,
 MaxConnIdle:       5 * time.Minute,
 StartupTimeout:    30 * time.Second,
 RunMigrations:     true,
 MigrationsPath:    "database/migrations",
}

tc, err := postgres.StartPostgreSQLContainer(ctx, config)
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `DatabaseName` | string | `"testdb"` | Name of the database to create |
| `Username` | string | `"testuser"` | Database username |
| `Password` | string | `"testpass"` | Database password |
| `PostgreSQLVersion` | string | `"16-3.4"` | PostgreSQL-PostGIS version (format: pg_version-postgis_version) |
| `MaxConns` | int32 | `10` | Maximum connections in pool |
| `MinConns` | int32 | `2` | Minimum connections in pool |
| `MaxConnLife` | time.Duration | `30m` | Maximum connection lifetime |
| `MaxConnIdle` | time.Duration | `5m` | Maximum connection idle time |
| `StartupTimeout` | time.Duration | `30s` | Container startup timeout |
| `RunMigrations` | bool | `false` | Whether to run migrations on startup |
| `MigrationsPath` | string | `""` | Path to migrations (auto-detected if empty) |

## PostGIS Support

The package uses the `postgis/postgis` image, which includes PostGIS extensions for geospatial queries:

```go
tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
if err != nil {
 t.Fatalf("Failed to start container: %v", err)
}
defer tc.Close()

// Create table with geography column
_, err = tc.Pool.Exec(ctx, `
 CREATE TABLE locations (
  id SERIAL PRIMARY KEY,
  name TEXT,
  location GEOGRAPHY(POINT)
 )
`)

// Insert geospatial data
_, err = tc.Pool.Exec(ctx, `
 INSERT INTO locations (name, location)
 VALUES ('San Francisco', ST_MakePoint(-122.4194, 37.7749))
`)

// Query with spatial functions
rows, err := tc.Pool.Query(ctx, `
 SELECT name
 FROM locations
 WHERE ST_DWithin(
  location,
  ST_MakePoint(-122.4, 37.7)::geography,
  10000  -- 10km radius
 )
`)
```

## Migration Support

### Automatic Migration Detection

The package automatically detects migration directories in your project:

```go
tc, err := postgres.StartPostgreSQLContainerWithMigrations(ctx, "")
// Searches for: database/migrations, migrations, sql/migrations, db/migrations
```

### Manual Migration Path

Specify a custom migration path:

```go
config := postgres.DefaultPostgreSQLConfig()
config.RunMigrations = true
config.MigrationsPath = "/path/to/migrations"

tc, err := postgres.StartPostgreSQLContainer(ctx, config)
```

### Environment Variable Override

Set `MIGRATIONS_PATH` environment variable:

```bash
export MIGRATIONS_PATH=/custom/migrations
```

### Migration File Format

Migrations use `golang-migrate` format:

```
migrations/
├── 001_create_users.up.sql
├── 001_create_users.down.sql
├── 002_add_posts.up.sql
└── 002_add_posts.down.sql
```

Example migration:

```sql
-- 001_create_users.up.sql
CREATE TABLE users (
 id SERIAL PRIMARY KEY,
 name TEXT NOT NULL,
 email TEXT UNIQUE NOT NULL,
 created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
```

## Test Isolation

### Clean All Tables

Remove all data from all tables (except system tables):

```go
func TestWithCleanup(t *testing.T) {
 tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
 if err != nil {
  t.Fatalf("Failed to start container: %v", err)
 }
 defer tc.Close()

 // Setup test data
 setupTestData(tc)

 // Run test
 runTest(tc)

 // Clean all tables for next test
 if err := tc.CleanAllTables(ctx); err != nil {
  t.Fatalf("Failed to clean tables: %v", err)
 }
}
```

### Clean Specific Tables

Remove data from specific tables only:

```go
func TestWithSpecificCleanup(t *testing.T) {
 tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
 if err != nil {
  t.Fatalf("Failed to start container: %v", err)
 }
 defer tc.Close()

 // Only clean specific tables
 if err := tc.CleanSpecificTables(ctx, "users", "posts"); err != nil {
  t.Fatalf("Failed to clean tables: %v", err)
 }
}
```

### Deferred Cleanup Pattern

Use helper functions for automatic cleanup:

```go
func TestWithDeferredCleanup(t *testing.T) {
 tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
 if err != nil {
  t.Fatalf("Failed to start container: %v", err)
 }
 defer tc.WithCleanup()()  // Closes container

 // Or clean specific tables
 defer tc.WithTableCleanup("users", "posts")()

 // Your test logic here
}
```

## Multiple Databases

Create multiple isolated databases within the same container:

```go
tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
if err != nil {
 t.Fatalf("Failed to start container: %v", err)
}
defer tc.Close()

// Create additional databases
db1URL, err := tc.NewTestDatabase("test_db1")
if err != nil {
 t.Fatalf("Failed to create database: %v", err)
}

db2URL, err := tc.NewTestDatabase("test_db2")
if err != nil {
 t.Fatalf("Failed to create database: %v", err)
}

// Each database is completely isolated
```

## Docker Availability Checking

### Skip Tests When Docker Unavailable

```go
func TestRequiresDocker(t *testing.T) {
 if shouldSkip, msg := postgres.SkipIfDockerUnavailable(); shouldSkip {
  t.Skip(msg)
 }

 // Test logic that requires Docker
}
```

### Check Docker Status

```go
result := postgres.CheckDockerAvailability()
if !result.Available {
 log.Printf("Docker unavailable: %s", result.Reason)
 if result.Error != nil {
  log.Printf("Error: %v", result.Error)
 }
}
```

## Error Handling

The package provides specific error types for common scenarios:

```go
tc, err := postgres.StartPostgreSQLContainerWithCheck(ctx, config)
if err != nil {
 switch {
 case errors.Is(err, postgres.ErrDockerNotAvailable):
  // Docker is not installed or not running
 case errors.Is(err, postgres.ErrContainerStartTimeout):
  // Container took too long to start
 case errors.Is(err, postgres.ErrContainerPortConflict):
  // Port is already in use
 case errors.Is(err, postgres.ErrDatabaseConnFailed):
  // Could not connect to database
 case errors.Is(err, postgres.ErrMigrationsFailed):
  // Database migrations failed
 default:
  // Other error
 }
}
```

## Troubleshooting

### Docker Not Available

**Error:** `Docker is not available or running`

**Solutions:**

1. Install Docker (see Installation section)
2. Start Docker daemon:
   - macOS: Open Docker Desktop
   - Linux: `sudo systemctl start docker`
   - Windows: Start Docker Desktop
3. Check Docker is running: `docker info`

### Container Startup Timeout

**Error:** `container failed to start within timeout period`

**Solutions:**

1. Increase startup timeout:

   ```go
   config := postgres.DefaultPostgreSQLConfig()
   config.StartupTimeout = 60 * time.Second
   ```

2. Check Docker resources (CPU, memory)
3. Pull image manually: `docker pull postgis/postgis:16-3.4`

### Port Conflict

**Error:** `container port conflict detected`

**Solutions:**

1. Stop conflicting containers: `docker ps` then `docker stop <container_id>`
2. Use dynamic port allocation (default behavior)
3. Check for other PostgreSQL instances

### Migration Failures

**Error:** `database migrations failed`

**Solutions:**

1. Verify migration files exist and are readable
2. Check migration SQL syntax
3. Set `MIGRATIONS_PATH` environment variable explicitly
4. Use absolute paths for migration directories

### Image Pull Failures

**Error:** `failed to start PostgreSQL container`

**Solutions:**

1. Check internet connectivity
2. Pull image manually: `docker pull postgis/postgis:16-3.4`
3. Check Docker Hub status
4. Use a different version if necessary

## Best Practices

### 1. Use Deferred Cleanup

Always defer cleanup to ensure containers are properly terminated:

```go
tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
if err != nil {
 t.Fatalf("Failed to start container: %v", err)
}
defer tc.Close()  // Always defer cleanup
```

### 2. Isolate Tests

Clean tables between tests for isolation:

```go
func TestSuite(t *testing.T) {
 tc, err := postgres.StartSimplePostgreSQLContainer(ctx)
 if err != nil {
  t.Fatalf("Failed to start container: %v", err)
 }
 defer tc.Close()

 t.Run("Test1", func(t *testing.T) {
  defer tc.CleanAllTables(ctx)
  // Test logic
 })

 t.Run("Test2", func(t *testing.T) {
  defer tc.CleanAllTables(ctx)
  // Test logic
 })
}
```

### 3. Reuse Containers in Test Suites

Start one container for the entire test suite:

```go
var testContainer *postgres.PostgreSQLTestContainer

func TestMain(m *testing.M) {
 ctx := context.Background()

 // Skip tests if Docker unavailable
 if shouldSkip, msg := postgres.SkipIfDockerUnavailable(); shouldSkip {
  fmt.Println(msg)
  os.Exit(0)
 }

 // Setup
 var err error
 testContainer, err = postgres.StartSimplePostgreSQLContainer(ctx)
 if err != nil {
  log.Fatalf("Failed to start container: %v", err)
 }

 // Run tests
 code := m.Run()

 // Teardown
 testContainer.Close()
 os.Exit(code)
}
```

### 4. Use Specific Cleanup When Possible

Clean only the tables you need:

```go
// Instead of cleaning all tables
tc.CleanAllTables(ctx)

// Clean only what you need
tc.CleanSpecificTables(ctx, "users", "posts")
```

### 5. Handle Docker Unavailable Gracefully

```go
func TestWithDocker(t *testing.T) {
 if shouldSkip, msg := postgres.SkipIfDockerUnavailable(); shouldSkip {
  t.Skip(msg)
 }

 // Test logic
}
```

## Examples

See the `examples/` directory for complete working examples:

- `examples/basic/` - Basic container usage
- `examples/migrations/` - Migration support
- `examples/cleanup/` - Table cleanup patterns

Run examples:

```bash
cd examples/basic && go run main.go
cd examples/migrations && go run main.go
cd examples/cleanup && go run main.go
```

## API Reference

### Functions

- `DefaultPostgreSQLConfig() *PostgreSQLConfig` - Returns default configuration
- `CheckDockerAvailability() DockerAvailabilityResult` - Checks Docker status
- `StartPostgreSQLContainer(ctx, config) (*PostgreSQLTestContainer, error)` - Starts container
- `StartPostgreSQLContainerWithCheck(ctx, config) (*PostgreSQLTestContainer, error)` - Starts with Docker check
- `StartSimplePostgreSQLContainer(ctx) (*PostgreSQLTestContainer, error)` - Starts with defaults
- `StartPostgreSQLContainerWithMigrations(ctx, path) (*PostgreSQLTestContainer, error)` - Starts with migrations
- `SkipIfDockerUnavailable() (bool, string)` - Helper for test skipping
- `FindMigrationsPath() string` - Auto-detects migration directory

### Methods

- `tc.Close() error` - Terminates container and closes pool
- `tc.CleanAllTables(ctx) error` - Truncates all tables
- `tc.CleanSpecificTables(ctx, tables...) error` - Truncates specific tables
- `tc.GetConnectionString() string` - Returns database URL
- `tc.GetPool() *pgxpool.Pool` - Returns connection pool
- `tc.GetContainer() *postgres.PostgresContainer` - Returns container
- `tc.NewTestDatabase(name) (string, error)` - Creates new database
- `tc.WithCleanup() func()` - Returns cleanup function
- `tc.WithTableCleanup(tables...) func()` - Returns table cleanup function

## License

MIT License - see LICENSE file for details

## Contributing

Contributions welcome! Please open an issue or pull request on GitHub.

## Repository

<https://github.com/JohnPlummer/go-testcontainers-postgres>
