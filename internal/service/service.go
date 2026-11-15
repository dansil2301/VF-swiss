package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	netmail "net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"log/slog"

	"github.com/vfswiss/vfswiss/internal/config"
	"github.com/vfswiss/vfswiss/internal/mail"
	"github.com/vfswiss/vfswiss/internal/models"
	"github.com/vfswiss/vfswiss/internal/repository"
)

type FileUpload struct {
	Data         []byte
	OriginalName string
	MIMEType     string
	Size         int64
}

type CreateRequestInput struct {
	Name        string
	Phone       string
	Email       string
	Brand       string
	Description string
	Photos      []FileUpload
	Honeypot    string
}

type Service struct {
	cfg        *config.Config
	repo       *repository.Repository
	mailClient *mail.Client
	logger     *slog.Logger
}

var (
	ErrInvalidName        = errors.New("invalid name")
	ErrInvalidPhone       = errors.New("invalid phone")
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidBrand       = errors.New("invalid brand")
	ErrInvalidDescription = errors.New("invalid description")
	ErrHoneypot           = errors.New("honeypot field filled")
)

func New(cfg *config.Config, repo *repository.Repository, mailClient *mail.Client, logger *slog.Logger) *Service {
	return &Service{cfg: cfg, repo: repo, mailClient: mailClient, logger: logger}
}

func (s *Service) CreateRepairRequest(ctx context.Context, input CreateRequestInput) (*models.Request, error) {
	name := sanitizeText(input.Name)
	phone := sanitizeText(input.Phone)
	email := sanitizeText(input.Email)
	brand := sanitizeText(input.Brand)
	description := sanitizeText(input.Description)

	if name == "" {
		return nil, ErrInvalidName
	}
	if len(name) > 100 {
		return nil, ErrInvalidName
	}
	if phone == "" {
		return nil, ErrInvalidPhone
	}
	if len(phone) < 7 || len(phone) > 30 || !validPhone(phone) {
		return nil, ErrInvalidPhone
	}
	if email != "" {
		if _, err := netmail.ParseAddress(email); err != nil {
			return nil, ErrInvalidEmail
		}
	}
	if brand == "" || len(brand) > 100 {
		return nil, ErrInvalidBrand
	}
	if description != "" && len(description) > 1200 {
		return nil, ErrInvalidDescription
	}
	if input.Honeypot != "" {
		return nil, ErrHoneypot
	}
	if len(input.Photos) > s.cfg.MaxUploadFiles {
		return nil, fmt.Errorf("too many files")
	}

	request := &models.Request{
		Name:        name,
		Phone:       phone,
		Email:       email,
		Brand:       brand,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}

	if err := s.repo.CreateRequest(ctx, request); err != nil {
		return nil, err
	}

	var photos []*models.RequestPhoto
	for _, upload := range input.Photos {
		if upload.Size == 0 {
			continue
		}
		filename, err := s.savePhoto(upload)
		if err != nil {
			return request, err
		}
		photo := &models.RequestPhoto{
			RequestID:    request.ID,
			Filename:     filename,
			OriginalName: upload.OriginalName,
			MIMEType:     upload.MIMEType,
			Size:         upload.Size,
			CreatedAt:    time.Now().UTC(),
		}
		if err := s.repo.CreateRequestPhoto(ctx, photo); err != nil {
			return request, err
		}
		photos = append(photos, photo)
	}

	if err := s.sendNotification(ctx, request, photos); err != nil {
		s.logger.Warn("smtp error", "error", err)
	}
	if err := s.sendUserConfirmation(ctx, request); err != nil {
		s.logger.Warn("smtp user confirmation error", "error", err)
	}

	return request, nil
}

func (s *Service) savePhoto(upload FileUpload) (string, error) {
	ext := strings.ToLower(filepath.Ext(upload.OriginalName))
	if ext == "" {
		switch upload.MIMEType {
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		default:
			ext = ".jpg"
		}
	}
	// If file bytes are provided, save to disk. Otherwise generate a reference string and do not store bytes.
	filename := fmt.Sprintf("%s_%d%s", randomToken(16), time.Now().Unix(), ext)
	if len(upload.Data) == 0 {
		// Do not save file bytes to disk — return generated reference only
		return "ref_" + filename, nil
	}
	path := filepath.Join(s.cfg.UploadDir, filename)

	if err := os.MkdirAll(s.cfg.UploadDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, upload.Data, 0o644); err != nil {
		return "", err
	}
	return filename, nil
}

func (s *Service) HealthCheck(ctx context.Context) error {
	return s.repo.DB().PingContext(ctx)
}

func (s *Service) sendNotification(ctx context.Context, request *models.Request, photos []*models.RequestPhoto) error {
	if s.mailClient == nil {
		return nil
	}
	return s.mailClient.SendRequestNotification(ctx, request, photos)
}

func (s *Service) sendUserConfirmation(ctx context.Context, request *models.Request) error {
	if s.mailClient == nil {
		return nil
	}
	return s.mailClient.SendUserConfirmation(ctx, request)
}

func sanitizeText(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, "\x00", "")
	trimmed = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, trimmed)
	return strings.Join(strings.Fields(trimmed), " ")
}

func validPhone(value string) bool {
	pattern := regexp.MustCompile(`^[0-9+()\s-]{7,30}$`)
	return pattern.MatchString(strings.TrimSpace(value))
}

func randomToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
