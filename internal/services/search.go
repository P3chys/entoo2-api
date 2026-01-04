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
		if _, createErr := client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        "documents",
			PrimaryKey: "id",
		}); createErr != nil {
			log.Printf("Failed to create meilisearch documents index: %v", createErr)
		}
	}

	// Configure documents index settings
	docIndex := client.Index("documents")

	// Configure filterable attributes
	_, err = docIndex.UpdateFilterableAttributes(&[]string{"subject_id", "mime_type", "category"})
	if err != nil {
		log.Printf("Failed to update filterable attributes: %v", err)
	}

	// Configure sortable attributes
	_, err = docIndex.UpdateSortableAttributes(&[]string{"created_at", "file_size"})
	if err != nil {
		log.Printf("Failed to update sortable attributes: %v", err)
	}

	// Configure searchable attributes with priorities
	_, err = docIndex.UpdateSearchableAttributes(&[]string{
		"original_name",  // Highest priority
		"content_text",   // Second priority
		"filename",       // Third priority
	})
	if err != nil {
		log.Printf("Failed to update searchable attributes: %v", err)
	}

	// Configure ranking rules for better relevance
	_, err = docIndex.UpdateRankingRules(&[]string{
		"words",           // Number of matched words
		"typo",            // Typo tolerance
		"proximity",       // Proximity of matched words
		"attribute",       // Order of searchable attributes
		"sort",            // Custom sorting
		"exactness",       // Exact matches first
	})
	if err != nil {
		log.Printf("Failed to update ranking rules: %v", err)
	}

	// Typo tolerance is enabled by default in Meilisearch with reasonable settings
	// Default: 1 typo for words >= 5 chars, 2 typos for words >= 9 chars

	// Ensure subjects index exists (best effort)
	_, err = client.GetIndex("subjects")
	if err != nil {
		if _, createErr := client.CreateIndex(&meilisearch.IndexConfig{
			Uid:        "subjects",
			PrimaryKey: "id",
		}); createErr != nil {
			log.Printf("Failed to create meilisearch subjects index: %v", createErr)
		}
	}

	// Configure subjects index settings
	subIndex := client.Index("subjects")

	// Configure filterable attributes for subjects
	_, err = subIndex.UpdateFilterableAttributes(&[]string{"semester_id", "code"})
	if err != nil {
		log.Printf("Failed to update subjects filterable attributes: %v", err)
	}

	// Configure sortable attributes for subjects
	_, err = subIndex.UpdateSortableAttributes(&[]string{"name_cs", "name_en", "created_at", "credits"})
	if err != nil {
		log.Printf("Failed to update subjects sortable attributes: %v", err)
	}

	// Configure searchable attributes for subjects with priorities
	_, err = subIndex.UpdateSearchableAttributes(&[]string{
		"code",           // Highest priority (exact course codes)
		"name_en",        // Second priority
		"name_cs",        // Third priority
		"description_en", // Fourth priority
		"description_cs", // Fifth priority
	})
	if err != nil {
		log.Printf("Failed to update subjects searchable attributes: %v", err)
	}

	// Configure ranking rules for subjects
	_, err = subIndex.UpdateRankingRules(&[]string{
		"words",
		"typo",
		"proximity",
		"attribute",
		"sort",
		"exactness",
	})
	if err != nil {
		log.Printf("Failed to update subjects ranking rules: %v", err)
	}

	// Note: Typo tolerance enabled by default for better search experience
	// Course codes prioritized via searchable attributes ranking

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

func (s *SearchService) Search(query string, subjectID string, mimeType string, exactMatch bool) (*meilisearch.SearchResponse, error) {
	request := &meilisearch.SearchRequest{
		Limit:                    50,
		AttributesToHighlight:    []string{"content_text", "original_name"},
		HighlightPreTag:          "<mark>",
		HighlightPostTag:         "</mark>",
		AttributesToCrop:         []string{"content_text"},
		CropLength:               200,
		ShowMatchesPosition:      true,
	}

	// Build filter conditions
	var filters []string
	if subjectID != "" {
		filters = append(filters, "subject_id = "+subjectID)
	}
	if mimeType != "" {
		filters = append(filters, "mime_type = "+mimeType)
	}

	if len(filters) > 0 {
		filterStr := filters[0]
		for i := 1; i < len(filters); i++ {
			filterStr += " AND " + filters[i]
		}
		request.Filter = filterStr
	}

	// Disable fuzzy matching for exact searches
	if exactMatch {
		request.MatchingStrategy = "all"
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

func (s *SearchService) SearchSubjects(query string, semesterID string, exactMatch bool) (*meilisearch.SearchResponse, error) {
	request := &meilisearch.SearchRequest{
		Limit:                    100,
		AttributesToHighlight:    []string{"name_cs", "name_en", "description_cs", "description_en", "code"},
		HighlightPreTag:          "<mark>",
		HighlightPostTag:         "</mark>",
		AttributesToCrop:         []string{"description_cs", "description_en"},
		CropLength:               200,
		ShowMatchesPosition:      true,
	}

	if semesterID != "" {
		request.Filter = "semester_id = " + semesterID
	}

	// Disable fuzzy matching for exact searches
	if exactMatch {
		request.MatchingStrategy = "all"
	}

	return s.client.Index("subjects").Search(query, request)
}

// SearchAll searches both documents and subjects and combines results
func (s *SearchService) SearchAll(query string, searchType string, subjectID string, mimeType string, exactMatch bool) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Search documents if type is "all" or "documents"
	if searchType == "" || searchType == "all" || searchType == "documents" {
		docResults, err := s.Search(query, subjectID, mimeType, exactMatch)
		if err != nil {
			return nil, err
		}
		result["documents"] = docResults.Hits
		result["documents_count"] = docResults.EstimatedTotalHits
	}

	// Search subjects if type is "all" or "subjects"
	if searchType == "" || searchType == "all" || searchType == "subjects" {
		subjectResults, err := s.SearchSubjects(query, "", exactMatch)
		if err != nil {
			return nil, err
		}
		result["subjects"] = subjectResults.Hits
		result["subjects_count"] = subjectResults.EstimatedTotalHits
	}

	return result, nil
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
