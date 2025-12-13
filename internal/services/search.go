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

	// Ensure index exists (best effort)
	_, err := client.GetIndex("documents")
	if err != nil {
		_, err = client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        "documents",
			PrimaryKey: "id",
		})
		if err != nil {
			log.Printf("Failed to create meilisearch index: %v", err)
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
