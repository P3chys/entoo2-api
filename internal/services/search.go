package services

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SearchService wraps PostgreSQL full-text search (tsvector / tsquery + pg_trgm).
type SearchService struct {
	db *gorm.DB
}

func NewSearchService(db *gorm.DB) *SearchService {
	return &SearchService{db: db}
}

// IndexDocument is a no-op: the content_tsv generated column updates automatically.
func (s *SearchService) IndexDocument(_ interface{}) error { return nil }

// IndexDocuments is a no-op.
func (s *SearchService) IndexDocuments(_ interface{}) error { return nil }

// DeleteDocument is a no-op: cascade handles DB deletion.
func (s *SearchService) DeleteDocument(_ string) error { return nil }

// IndexSubject is a no-op.
func (s *SearchService) IndexSubject(_ interface{}) error { return nil }

// IndexSubjects is a no-op.
func (s *SearchService) IndexSubjects(_ interface{}) error { return nil }

// DeleteSubject is a no-op.
func (s *SearchService) DeleteSubject(_ string) error { return nil }

// GetDocumentCount returns the total number of documents in the DB.
func (s *SearchService) GetDocumentCount() (int64, error) {
	var count int64
	err := s.db.Raw("SELECT COUNT(*) FROM documents").Scan(&count).Error
	return count, err
}

// pgDocRow is used to scan raw search query results.
type pgDocRow struct {
	ID              string    `gorm:"column:id"`
	SubjectID       string    `gorm:"column:subject_id"`
	UploadedBy      string    `gorm:"column:uploaded_by"`
	TypeID          *string   `gorm:"column:type_id"`
	CategoryID      *string   `gorm:"column:category_id"`
	Filename        string    `gorm:"column:filename"`
	OriginalName    string    `gorm:"column:original_name"`
	FileSize        int64     `gorm:"column:file_size"`
	MimeType        string    `gorm:"column:mime_type"`
	FilePath        string    `gorm:"column:file_path"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	ContentHeadline string    `gorm:"column:content_headline"`
	NameHeadline    string    `gorm:"column:name_headline"`
}

// pgSubjectRow is used to scan raw subject search results.
type pgSubjectRow struct {
	ID            string    `gorm:"column:id"`
	SemesterID    string    `gorm:"column:semester_id"`
	NameCS        string    `gorm:"column:name_cs"`
	DescriptionCS string    `gorm:"column:description_cs"`
	Credits       int       `gorm:"column:credits"`
	Code          string    `gorm:"column:code"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
	NameHeadline  string    `gorm:"column:name_headline"`
	DescHeadline  string    `gorm:"column:desc_headline"`
}

const headlineOpts = "StartSel=<mark>, StopSel=</mark>, MaxFragments=1, MaxWords=50"
const nameHeadlineOpts = "StartSel=<mark>, StopSel=</mark>"

// searchDocuments runs FTS + trigram fallback on the documents table.
func (s *SearchService) searchDocuments(query, subjectID, mimeType string, exactMatch bool) ([]map[string]interface{}, int64, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, 0, nil
	}

	tsFunc := "plainto_tsquery"
	if exactMatch {
		tsFunc = "phraseto_tsquery"
	}
	tsq := fmt.Sprintf("%s('simple', ?)", tsFunc)

	sql := fmt.Sprintf(`
		SELECT
			d.id, d.subject_id, d.uploaded_by, d.type_id, d.category_id,
			d.filename, d.original_name, d.file_size, d.mime_type, d.file_path, d.created_at,
			ts_headline('simple', COALESCE(d.content_text, ''), %s, '%s') AS content_headline,
			ts_headline('simple', d.original_name,              %s, '%s') AS name_headline
		FROM documents d
		WHERE (
			d.content_tsv @@ %s
			OR d.original_name ILIKE '%%' || ? || '%%'
		)
		AND (? = '' OR d.subject_id::text = ?)
		AND (? = '' OR d.mime_type = ?)
		ORDER BY ts_rank_cd(d.content_tsv, %s) DESC
		LIMIT 50
	`, tsq, headlineOpts, tsq, nameHeadlineOpts, tsq, tsq)

	// args: tsq×5 for the SELECT headlines + WHERE, then ilike, subjectID×2, mimeType×2, tsq for ORDER BY
	args := []interface{}{q, q, q, q, subjectID, subjectID, mimeType, mimeType, q}

	var rows []pgDocRow
	if err := s.db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	hits := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		formatted := map[string]string{}
		if r.ContentHeadline != "" {
			formatted["content_text"] = r.ContentHeadline
		}
		if r.NameHeadline != "" {
			formatted["original_name"] = r.NameHeadline
		}
		hit := map[string]interface{}{
			"id":            r.ID,
			"subject_id":    r.SubjectID,
			"uploaded_by":   r.UploadedBy,
			"type_id":       r.TypeID,
			"category_id":   r.CategoryID,
			"filename":      r.Filename,
			"original_name": r.OriginalName,
			"file_size":     r.FileSize,
			"mime_type":     r.MimeType,
			"file_path":     r.FilePath,
			"created_at":    r.CreatedAt,
			"_formatted":    formatted,
		}
		hits = append(hits, hit)
	}

	return hits, int64(len(hits)), nil
}

// searchSubjects runs FTS + ILIKE fallback on the subjects table.
func (s *SearchService) searchSubjects(query, _ string, exactMatch bool) ([]map[string]interface{}, int64, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, 0, nil
	}

	tsFunc := "plainto_tsquery"
	if exactMatch {
		tsFunc = "phraseto_tsquery"
	}
	tsq := fmt.Sprintf("%s('simple', ?)", tsFunc)

	sql := fmt.Sprintf(`
		SELECT
			s.id, s.semester_id, s.name_cs, s.description_cs, s.credits,
			COALESCE(s.code, '') as code,
			s.created_at, s.updated_at,
			ts_headline('simple', s.name_cs,                        %s, '%s') AS name_headline,
			ts_headline('simple', COALESCE(s.description_cs, ''),   %s, '%s') AS desc_headline
		FROM subjects s
		WHERE
			to_tsvector('simple', s.name_cs || ' ' || COALESCE(s.description_cs, '')) @@ %s
			OR s.name_cs ILIKE '%%' || ? || '%%'
		ORDER BY
			ts_rank_cd(
				to_tsvector('simple', s.name_cs || ' ' || COALESCE(s.description_cs, '')),
				%s
			) DESC
		LIMIT 100
	`, tsq, nameHeadlineOpts, tsq, headlineOpts, tsq, tsq)

	args := []interface{}{q, q, q, q, q}

	var rows []pgSubjectRow
	if err := s.db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	hits := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		formatted := map[string]string{}
		if r.NameHeadline != "" {
			formatted["name_cs"] = r.NameHeadline
		}
		if r.DescHeadline != "" {
			formatted["description_cs"] = r.DescHeadline
		}
		hit := map[string]interface{}{
			"id":             r.ID,
			"semester_id":    r.SemesterID,
			"name_cs":        r.NameCS,
			"description_cs": r.DescriptionCS,
			"credits":        r.Credits,
			"code":           r.Code,
			"created_at":     r.CreatedAt,
			"updated_at":     r.UpdatedAt,
			"_formatted":     formatted,
		}
		hits = append(hits, hit)
	}

	return hits, int64(len(hits)), nil
}

// SearchAll searches documents and/or subjects depending on searchType.
func (s *SearchService) SearchAll(query, searchType, subjectID, mimeType string, exactMatch bool) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	if searchType == "" || searchType == "all" || searchType == "documents" {
		docs, count, err := s.searchDocuments(query, subjectID, mimeType, exactMatch)
		if err != nil {
			return nil, err
		}
		result["documents"] = docs
		result["documents_count"] = count
	}

	if searchType == "" || searchType == "all" || searchType == "subjects" {
		subjects, count, err := s.searchSubjects(query, "", exactMatch)
		if err != nil {
			return nil, err
		}
		result["subjects"] = subjects
		result["subjects_count"] = count
	}

	return result, nil
}
