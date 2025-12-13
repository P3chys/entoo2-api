package services

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/P3chys/entoo2-api/internal/config"
)

type TextExtractionService struct {
	tikaURL string
}

func NewTextExtractionService(cfg *config.Config) *TextExtractionService {
	return &TextExtractionService{
		tikaURL: cfg.TikaURL,
	}
}

func (s *TextExtractionService) ExtractText(file multipart.File) (string, error) {
	// Need to seek to start of file as it might have been read by storage service
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, 0)
	}

	req, err := http.NewRequest("PUT", s.tikaURL+"/tika", file)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(body)), nil
}
