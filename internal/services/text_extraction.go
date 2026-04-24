package services

import (
	"archive/zip"
	"io"
	"mime/multipart"
	"path"
	"regexp"
	"strings"

	"github.com/P3chys/entoo2-api/internal/config"
)

// TextExtractionService extracts plain text from uploaded documents using pure Go.
// Supported formats: DOCX, PPTX, XLSX, TXT, CSV.
// PDF text extraction is not supported; PDFs will have an empty content_text.
type TextExtractionService struct{}

func NewTextExtractionService(_ *config.Config) *TextExtractionService {
	return &TextExtractionService{}
}

var xmlTagRe = regexp.MustCompile(`<[^>]*>`)

// ExtractText returns plain text for indexing. mimeType selects the parser.
func (s *TextExtractionService) ExtractText(file multipart.File, mimeType string) (string, error) {
	if seeker, ok := file.(io.Seeker); ok {
		_, _ = seeker.Seek(0, io.SeekStart)
	}

	switch mimeType {
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return extractZipXML(file, []string{"word/document.xml"})
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return extractZipXML(file, []string{"ppt/slides/slide*.xml"})
	case "application/vnd.ms-powerpoint":
		return extractZipXML(file, []string{"ppt/slides/slide*.xml"})
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-excel":
		return extractZipXML(file, []string{"xl/sharedStrings.xml"})
	case "text/plain", "text/csv":
		data, err := io.ReadAll(file)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	default:
		// PDF and other binary formats: no extraction
		return "", nil
	}
}

// extractZipXML opens the file as a ZIP archive, reads entries matching any of
// the given glob patterns (using forward-slash path.Match), strips XML tags, and
// returns concatenated plain text.
func extractZipXML(file multipart.File, patterns []string) (string, error) {
	// Determine size
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	zr, err := zip.NewReader(file, size)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, f := range zr.File {
		matched := false
		for _, pat := range patterns {
			if ok, _ := path.Match(pat, f.Name); ok {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()

		text := xmlTagRe.ReplaceAllString(string(data), " ")
		words := strings.Fields(text)
		sb.WriteString(strings.Join(words, " "))
		sb.WriteByte(' ')
	}

	return strings.TrimSpace(sb.String()), nil
}
