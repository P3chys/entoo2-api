package services

import (
	"encoding/json"

	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ActivityService struct {
	db *gorm.DB
}

func NewActivityService(db *gorm.DB) *ActivityService {
	return &ActivityService{
		db: db,
	}
}

func (s *ActivityService) CreateActivity(userID uuid.UUID, activityType models.ActivityType, subjectID, documentID *uuid.UUID, metadata map[string]interface{}) error {
	metadataJSON := "{}"
	if len(metadata) > 0 {
		bytes, err := json.Marshal(metadata)
		if err == nil {
			metadataJSON = string(bytes)
		}
	}

	activity := models.Activity{
		UserID:       userID,
		ActivityType: activityType,
		SubjectID:    subjectID,
		DocumentID:   documentID,
		Metadata:     metadataJSON,
	}

	return s.db.Create(&activity).Error
}

func (s *ActivityService) GetRecentActivities(limit int) ([]models.Activity, error) {
	var activities []models.Activity
	err := s.db.Preload("User").Preload("Subject").Preload("Document").
		Order("created_at desc").
		Limit(limit).
		Find(&activities).Error
	return activities, err
}
