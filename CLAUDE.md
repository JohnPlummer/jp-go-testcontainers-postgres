# CLAUDE.md

Configuration for Claude Code when working with jp-go-testcontainers-postgres package.

## Standards

Use `/ai-common` skill to load development standards and patterns as needed.

## Package Purpose

jp-go-testcontainers-postgres provides PostgreSQL testcontainer utilities for Go projects with:

- Test database lifecycle management
- Automatic container cleanup
- Connection string generation
- Integration with jp-go-pgx-utils
- Migration support
- Consistent test database setup

## Development Guidelines

This is a **shared package** used across multiple projects. Changes must be:

- Backward compatible
- Well-tested
- Generic (not project-specific)
- Documented in examples
