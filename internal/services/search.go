package services

import (
	"log"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/P3chys/entoo2-api/internal/models"
	"github.com/meilisearch/meilisearch-go"
)

type SearchService struct {
	client *meilisearch.Client
	index  string
}

func NewSearchService(cfg *config.Config) *SearchService {
	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   cfg.MeiliURL,
		APIKey: cfg.MeiliAPIKey,
	})

	// Ensure documents index exists (best effort)
	_, err := client.GetIndex("documents")
	if err != nil {
		_, err = client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        "documents",
			PrimaryKey: "id",
		})
		if err != nil {
			log.Printf("Failed to create meilisearch documents index: %v", err)
		}

		// Configure filterable attributes
		_, err = client.Index("documents").UpdateFilterableAttributes(&[]string{"subject_id", "mime_type"})
		if err != nil {
			log.Printf("Failed to update filterable attributes: %v", err)
		}

		// Configure sortable attributes
		_, err = client.Index("documents").UpdateSortableAttributes(&[]string{"created_at"})
		if err != nil {
			log.Printf("Failed to update sortable attributes: %v", err)
		}
	}

	// Ensure subjects index exists (best effort)
	_, err = client.GetIndex("subjects")
	if err != nil {
		_, err = client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        "subjects",
			PrimaryKey: "id",
		})
		if err != nil {
			log.Printf("Failed to create meilisearch subjects index: %v", err)
		}

		// Configure filterable attributes for subjects
		_, err = client.Index("subjects").UpdateFilterableAttributes(&[]string{"semester_id", "code"})
		if err != nil {
			log.Printf("Failed to update subjects filterable attributes: %v", err)
		}

		// Configure sortable attributes for subjects
		_, err = client.Index("subjects").UpdateSortableAttributes(&[]string{"name_cs", "name_en", "created_at"})
		if err != nil {
			log.Printf("Failed to update subjects sortable attributes: %v", err)
		}

		// Configure searchable attributes for subjects
		_, err = client.Index("subjects").UpdateSearchableAttributes(&[]string{"name_cs", "name_en", "code", "description_cs", "description_en"})
		if err != nil {
			log.Printf("Failed to update subjects searchable attributes: %v", err)
		}
	}

	return &SearchService{
		client: client,
		index:  "documents",
	}
}

func (s *SearchService) IndexDocument(doc models.Document) error {
	// Meilisearch accepts a list of documents
	_, err := s.client.Index(s.index).AddDocuments([]models.Document{doc})
	return err
}

func (s *SearchService) DeleteDocument(docID string) error {
	_, err := s.client.Index(s.index).DeleteDocument(docID)
	return err
}

func (s *SearchService) Search(query string, subjectID string) (*meilisearch.SearchResponse, error) {
	request := &meilisearch.SearchRequest{
		Limit: 20,
	}

	if subjectID != "" {
		request.Filter = "subject_id = " + subjectID
	}

	return s.client.Index(s.index).Search(query, request)
}

func (s *SearchService) IndexSubject(subject models.Subject) error {
	_, err := s.client.Index("subjects").AddDocuments([]models.Subject{subject})
	return err
}

func (s *SearchService) IndexSubjects(subjects []models.Subject) error {
	if len(subjects) == 0 {
		return nil
	}
	_, err := s.client.Index("subjects").AddDocuments(subjects)
	return err
}

func (s *SearchService) DeleteSubject(subjectID string) error {
	_, err := s.client.Index("subjects").DeleteDocument(subjectID)
	return err
}

func (s *SearchService) SearchSubjects(query string, semesterID string) (*meilisearch.SearchResponse, error) {
	request := &meilisearch.SearchRequest{
		Limit: 100,
	}

	if semesterID != "" {
		request.Filter = "semester_id = " + semesterID
	}

	return s.client.Index("subjects").Search(query, request)
}

func (s *SearchService) IndexDocuments(docs []models.Document) error {
	if len(docs) == 0 {
		return nil
	}
	_, err := s.client.Index(s.index).AddDocuments(docs)
	return err
}

func (s *SearchService) GetDocumentCount() (int64, error) {
	stats, err := s.client.Index(s.index).GetStats()
	if err != nil {
		return 0, err
	}
	return stats.NumberOfDocuments, nil
}
