# Entoo2 API

Backend REST API for the Entoo2 document-sharing platform.

## Tech Stack

- Go 1.22+
- Gin Web Framework
- GORM (PostgreSQL)
- JWT Authentication
- Redis (Cache/Sessions)
- MinIO (S3-compatible storage)
- Meilisearch (Full-text search)

## Quick Start

```bash
# Install dependencies
go mod download

# Copy environment file
cp .env.example .env

# Run migrations
make migrate-up

# Start development server
make dev
```

## Documentation

See the [Wiki](https://github.com/P3chys/entoo2-api/wiki) for detailed documentation.

## Project Structure

```
cmd/server/         # Application entrypoint
internal/
  ├── config/       # Configuration loading
  ├── database/     # DB connection & migrations
  ├── handlers/     # HTTP request handlers
  ├── middleware/   # HTTP middleware
  ├── models/       # Database models
  ├── repository/   # Data access layer
  ├── router/       # Route definitions
  └── services/     # Business logic
pkg/                # Shared packages
tests/              # Integration tests
```

## License

MIT
