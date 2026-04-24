package services

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/P3chys/entoo2-api/internal/config"
)

type StorageService struct {
	storagePath string
}

func NewStorageService(cfg *config.Config) (*StorageService, error) {
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory %q: %w", cfg.StoragePath, err)
	}
	return &StorageService{storagePath: cfg.StoragePath}, nil
}

func (s *StorageService) UploadFile(file multipart.File, filename string, _ int64, _ string) error {
	dst, err := os.Create(filepath.Join(s.storagePath, filename))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

// DownloadFile opens the stored file and returns a ReadCloser and its size.
func (s *StorageService) DownloadFile(filename string) (io.ReadCloser, int64, error) {
	path := filepath.Join(s.storagePath, filename)
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("file not found: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("failed to stat file: %w", err)
	}

	return f, info.Size(), nil
}

func (s *StorageService) DeleteFile(filename string) error {
	path := filepath.Join(s.storagePath, filename)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil // already gone, not an error
	}
	return err
}
