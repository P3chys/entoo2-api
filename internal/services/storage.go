package services

import (
	"context"
	"mime/multipart"

	"github.com/P3chys/entoo2-api/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type StorageService struct {
	client *minio.Client
	bucket string
}

func NewStorageService(cfg *config.Config) (*StorageService, error) {
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseSSL,
	})
	if err != nil {
		return nil, err
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.MinIOBucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		err = client.MakeBucket(ctx, cfg.MinIOBucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, err
		}
	}

	return &StorageService{
		client: client,
		bucket: cfg.MinIOBucket,
	}, nil
}

func (s *StorageService) UploadFile(file multipart.File, filename string, size int64, contentType string) error {
	ctx := context.Background()
	_, err := s.client.PutObject(ctx, s.bucket, filename, file, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *StorageService) UploadFileFromPath(ctx context.Context, file interface{}, filename string, size int64, contentType string) (minio.UploadInfo, error) {
	return s.client.PutObject(ctx, s.bucket, filename, file.(interface{ Read([]byte) (int, error) }), size, minio.PutObjectOptions{
		ContentType: contentType,
	})
}

func (s *StorageService) DownloadFile(filename string) (*minio.Object, error) {
	ctx := context.Background()
	return s.client.GetObject(ctx, s.bucket, filename, minio.GetObjectOptions{})
}

func (s *StorageService) DeleteFile(filename string) error {
	ctx := context.Background()
	return s.client.RemoveObject(ctx, s.bucket, filename, minio.RemoveObjectOptions{})
}
