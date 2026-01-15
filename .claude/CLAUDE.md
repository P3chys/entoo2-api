# Entoo2 API - Claude Code Context

## Project Overview

This is the backend REST API for Entoo2, a document-sharing web application for law students.

## Technology Stack

- **Language:** Go 1.22+
- **Framework:** Gin (HTTP router)
- **ORM:** GORM
- **Database:** PostgreSQL 16
- **Cache:** Redis 7
- **Storage:** MinIO (S3-compatible)
- **Search:** Meilisearch
- **Content Extraction:** Apache Tika

## Project Structure

```
entoo2-api/
├── cmd/
│   ├── server/              # Main API server
│   └── reindex-documents/   # Document reindexing tool
├── internal/
│   ├── config/              # Configuration management
│   ├── database/            # Database connection and seeding
│   ├── handlers/            # HTTP request handlers
│   ├── middleware/          # HTTP middleware (auth, etc.)
│   ├── models/              # Data models (GORM)
│   ├── services/            # Business logic
│   └── utils/               # Utilities and helpers
├── templates/
│   └── emails/              # Email templates (Czech/English)
├── Dockerfile               # Production container
├── Dockerfile.dev           # Development container
└── Makefile                 # Build and dev commands

```

## Claude Code Skills

**Available skills:**
- `/build` - Build the API server
- `/run` - Run in development mode
- `/test` - Run all tests with coverage
- `/lint` - Run linter checks

See parent project's [USAGE_GUIDE.md](../entoo2-infra/.claude/USAGE_GUIDE.md) for full documentation.

## Key Features

1. **Authentication:** JWT-based (access + refresh tokens)
2. **Document Management:** Upload, download, search PDFs and Office files
3. **Full-text Search:** Meilisearch integration with content extraction
4. **User Engagement:** Comments, Q&A, favorites
5. **Internationalization:** Czech and English support

## API Endpoints

Main endpoint groups:
- `/api/v1/auth` - Authentication (login, register, refresh)
- `/api/v1/users` - User management
- `/api/v1/semesters` - Academic semester management
- `/api/v1/subjects` - Course/subject management
- `/api/v1/documents` - Document CRUD and search
- `/api/v1/comments` - Subject comments
- `/api/v1/questions` - Q&A system
- `/api/v1/favorites` - User favorites
- `/api/v1/activities` - Activity feed

## Database Models

Main models (in `internal/models/`):
- `User` - User accounts
- `Semester` - Academic periods
- `Subject` - Courses within semesters
- `Document` - Uploaded files with metadata
- `Comment` - Subject-level comments
- `Question` / `Answer` - Q&A
- `Favorite` - User bookmarks
- `Activity` - User activity tracking

## Coding Conventions

- Follow standard Go project layout
- Use GORM for all database operations
- Structured logging with appropriate levels
- Error wrapping with context
- Handlers should be thin - business logic in services
- Write tests for all handlers and services
- Use meaningful variable names
- Comments for exported functions

## Environment Variables

Key configuration (see `.env` or config):
- `DATABASE_URL` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string
- `JWT_SECRET` - Token signing secret
- `MINIO_ENDPOINT` - Object storage URL
- `MEILI_URL` - Meilisearch URL
- `TIKA_URL` - Apache Tika URL

## Development Workflow

1. **Start infrastructure** (from entoo2-infra):
   ```bash
   /start
   ```

2. **Run API server**:
   ```bash
   /run
   ```

3. **Run tests**:
   ```bash
   /test
   ```

4. **Build for production**:
   ```bash
   /build
   ```

## Common Tasks

**Add new endpoint:**
1. Create handler in `internal/handlers/`
2. Register route in `cmd/server/main.go`
3. Add tests
4. Update API documentation

**Add new model:**
1. Create model in `internal/models/`
2. Add GORM tags for database mapping
3. Update migration/seed if needed
4. Create service layer functions

**Update database:**
1. Modify model structs
2. GORM auto-migrates on startup (dev mode)
3. For production, create manual migrations

## Related Projects

- `entoo2-infra` - Infrastructure and Docker setup
- `entoo2-web` - SvelteKit frontend

## Useful Commands

```bash
# Run server
go run cmd/server/main.go

# Build binary
go build -o bin/server cmd/server/main.go

# Run tests
go test ./... -v

# Run with coverage
go test ./... -cover

# Format code
go fmt ./...

# Vet code
go vet ./...

# Reindex documents
go run cmd/reindex-documents/main.go
```

## Dependencies

Main dependencies:
- `github.com/gin-gonic/gin` - HTTP framework
- `gorm.io/gorm` - ORM
- `github.com/golang-jwt/jwt` - JWT handling
- `github.com/minio/minio-go` - MinIO client
- `github.com/go-redis/redis` - Redis client

Install with:
```bash
go mod download
```
