package handlers

import (
	"database/sql"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setupMockDB creates a GORM DB instance backed by sqlmock for testing
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)

	dialector := postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "postgres",
	})

	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)

	return db, mock, sqlDB
}

func TestHealthCheck_Healthy(t *testing.T) {
	// Setup
	db, mock, sqlDB := setupMockDB(t)
	defer sqlDB.Close()

	// Expect ping to succeed
	mock.ExpectPing()

	router := setupTestRouter()
	router.GET("/health", HealthCheck(db))

	// Execute
	w := performRequest(router, "GET", "/health", nil)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	response := parseJSONResponse(w)
	assert.Equal(t, "healthy", response["status"])
	assert.Equal(t, "ok", response["database"])
	assert.Equal(t, "ok", response["cache"])
	assert.Equal(t, "ok", response["storage"])
	assert.Equal(t, "ok", response["search"])

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHealthCheck_DatabaseUnreachable(t *testing.T) {
	// Setup
	db, mock, sqlDB := setupMockDB(t)
	defer sqlDB.Close()

	// Expect ping to fail
	mock.ExpectPing().WillReturnError(sql.ErrConnDone)

	router := setupTestRouter()
	router.GET("/health", HealthCheck(db))

	// Execute
	w := performRequest(router, "GET", "/health", nil)

	// Assert
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	response := parseJSONResponse(w)
	assert.Equal(t, "unhealthy", response["status"])
	assert.Equal(t, "unreachable", response["database"])

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}
