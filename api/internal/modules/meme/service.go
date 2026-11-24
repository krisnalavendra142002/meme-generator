package meme

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/nyxsr/meme-portal/api/internal/config"
	"github.com/nyxsr/meme-portal/api/internal/platform/token"
)

// Gate memeriksa apakah user berhak membuat meme (langganan aktif).
// Diimplementasikan oleh modul subscription lewat structural typing.
type Gate interface {
	CanCreate(ctx context.Context, userID string) error
}

type Service struct {
	repo    *Repository
	s3      *s3.Client
	bucket  string
	baseURL string // public base URL Garage web endpoint
	gate    Gate
}

func NewService(repo *Repository, s3Client *s3.Client, cfg config.StorageConfig, gate Gate) *Service {
	return &Service{repo: repo, s3: s3Client, bucket: cfg.Bucket, baseURL: cfg.PublicBaseURL, gate: gate}
}

// PrepareUpload membuat record meme (pending) dan mengembalikan presigned PUT URL
// untuk mengunggah gambar dasar langsung ke S3/Garage.
func (s *Service) PrepareUpload(ctx context.Context, userID, title, ext, contentType string, size int64) (UploadResponse, error) {
	if err := s.gate.CanCreate(ctx, userID); err != nil {
		return UploadResponse{}, err
	}
	if !allowedExt(ext) {
		return UploadResponse{}, ErrInvalidFile
	}
	if size < 0 || size > maxUploadSize {
		return UploadResponse{}, ErrInvalidFile
	}
	mime, err := canonicalContentType(ext, contentType)
	if err != nil {
		return UploadResponse{}, ErrInvalidFile
	}

	objectKey := fmt.Sprintf("base/%s%s", token.RandomHex(12), strings.ToLower(ext))

	m, err := s.repo.Create(ctx, Meme{
		UserID: userID, Title: title,
		BaseImageURL: s.publicURL(objectKey),
	})
	if err != nil {
		return UploadResponse{}, err
	}

	presign := s3.NewPresignClient(s.s3)
	input := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	}
	if contentType != "" && mime != "" {
		input.ContentType = &mime
	}
	if size > 0 {
		input.ContentLength = &size
	}
	req, err := presign.PresignPutObject(ctx, input, s3.WithPresignExpires(maxPresignTTL))
	if err != nil {
		return UploadResponse{}, fmt.Errorf("meme service: presign gagal: %w", err)
	}
	return UploadResponse{ID: m.ID, UploadURL: req.URL, BaseURL: m.BaseImageURL}, nil
}

// ResultUpload mengembalikan presigned PUT URL untuk gambar hasil render
// (canvas) milik meme <id>; complete kemudian menyimpan URL publiknya.
func (s *Service) ResultUpload(ctx context.Context, userID, id string) (UploadResponse, error) {
	if _, err := s.repo.GetByID(ctx, id, userID); err != nil {
		return UploadResponse{}, err
	}
	objectKey := fmt.Sprintf("result/%s.png", id)

	presign := s3.NewPresignClient(s.s3)
	req, err := presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &objectKey,
		ContentType: &[]string{"image/png"}[0],
	}, s3.WithPresignExpires(maxPresignTTL))
	if err != nil {
		return UploadResponse{}, fmt.Errorf("meme service: presign gagal: %w", err)
	}
	return UploadResponse{ID: id, UploadURL: req.URL, BaseURL: s.publicURL(objectKey)}, nil
}

func (s *Service) Complete(ctx context.Context, userID, id, resultURL string) (Meme, error) {
	return s.repo.SetResultURL(ctx, id, userID, resultURL)
}

func (s *Service) Get(ctx context.Context, userID, id string) (Meme, error) {
	return s.repo.GetByID(ctx, id, userID)
}

func (s *Service) List(ctx context.Context, userID string, limit, offset int32) (ListResponse, error) {
	items, total, err := s.repo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		return ListResponse{}, err
	}
	if items == nil {
		items = []Meme{}
	}
	return ListResponse{Items: items, Total: total}, nil
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, id, userID)
}

func (s *Service) publicURL(key string) string {
	if s.baseURL == "" {
		return key
	}
	return strings.TrimRight(s.baseURL, "/") + "/" + path.Clean(key)
}

func allowedExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

func canonicalContentType(ext, clientContentType string) (string, error) {
	ct := strings.ToLower(strings.TrimSpace(clientContentType))
	if ct != "" {
		if strings.Contains(ct, "html") || strings.Contains(ct, "javascript") || strings.Contains(ct, "executable") || strings.Contains(ct, "zip") {
			return "", ErrInvalidFile
		}
	}
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		if ct != "" && ct != "image/jpeg" && ct != "image/jpg" && ct != "image/pjpeg" && ct != "application/octet-stream" {
			return "", ErrInvalidFile
		}
		return "image/jpeg", nil
	case ".png":
		if ct != "" && ct != "image/png" && ct != "application/octet-stream" {
			return "", ErrInvalidFile
		}
		return "image/png", nil
	case ".gif":
		if ct != "" && ct != "image/gif" && ct != "application/octet-stream" {
			return "", ErrInvalidFile
		}
		return "image/gif", nil
	case ".webp":
		if ct != "" && ct != "image/webp" && ct != "application/octet-stream" {
			return "", ErrInvalidFile
		}
		return "image/webp", nil
	default:
		return "", ErrInvalidFile
	}
}
