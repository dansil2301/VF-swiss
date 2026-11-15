package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               string
	DBPath             string
	UploadDir          string
	StaticDir          string
	BaseURL            string
	SMTPHost           string
	SMTPPort           int
	SMTPUser           string
	SMTPPass           string
	OwnerEmail         string
	MailFrom           string
	MaxUploadSize      int64
	MaxUploadTotal     int64
	MaxUploadFiles     int
	TurnstileSiteKey   string
	TurnstileSecretKey string
	RateLimitCount     int
	RateLimitWindow    time.Duration
	AllowedMIMETypes   []string
}

func New() (*Config, error) {
	_ = loadDotEnv(".env")
	port := getenv("PORT", "8080")
	dbPath := getenv("DB_PATH", "./database/app.db")
	uploadDir := getenv("UPLOAD_DIR", "./web/uploads")
	staticDir := getenv("STATIC_DIR", "./web/static")
	baseURL := getenv("BASE_URL", "https://vf-swiss.ru")
	smtpHost := getenv("SMTP_HOST", "")
	smtpPort := int(parseInt(getenv("SMTP_PORT", "465")))
	smtpUser := getenv("SMTP_USER", "")
	smtpPass := getenv("SMTP_PASS", "")
	ownerEmail := getenv("OWNER_EMAIL", "")
	mailFrom := getenv("MAIL_FROM", smtpUser)
	if mailFrom == "" {
		mailFrom = ownerEmail
	}
	maxUploadSize := parseInt64(getenv("MAX_UPLOAD_SIZE", "5242880"))
	maxUploadTotal := parseInt64(getenv("MAX_UPLOAD_TOTAL", "20971520"))
	maxUploadFiles := int(parseInt(getenv("MAX_UPLOAD_FILES", "5")))
	rateLimitCount := int(parseInt(getenv("RATE_LIMIT_COUNT", "5")))
	rateLimitWindow := parseDuration(getenv("RATE_LIMIT_WINDOW", "10m"))
	allowedMIMETypes := []string{"image/jpeg", "image/png", "image/webp"}

	if ownerEmail == "" {
		return nil, fmt.Errorf("OWNER_EMAIL is required")
	}
	turnstileSiteKey := os.Getenv("TURNSTILE_SITE_KEY")
	turnstileSecretKey := os.Getenv("TURNSTILE_SECRET_KEY")
	if (turnstileSiteKey == "") != (turnstileSecretKey == "") {
		return nil, fmt.Errorf("TURNSTILE_SITE_KEY and TURNSTILE_SECRET_KEY must both be set")
	}

	return &Config{
		Port:               port,
		DBPath:             dbPath,
		UploadDir:          uploadDir,
		StaticDir:          staticDir,
		BaseURL:            strings.TrimSuffix(baseURL, "/"),
		SMTPHost:           smtpHost,
		SMTPPort:           smtpPort,
		SMTPUser:           smtpUser,
		SMTPPass:           smtpPass,
		OwnerEmail:         ownerEmail,
		MailFrom:           mailFrom,
		MaxUploadSize:      maxUploadSize,
		MaxUploadTotal:     maxUploadTotal,
		MaxUploadFiles:     maxUploadFiles,
		TurnstileSiteKey:   turnstileSiteKey,
		TurnstileSecretKey: turnstileSecretKey,
		RateLimitCount:     rateLimitCount,
		RateLimitWindow:    rateLimitWindow,
		AllowedMIMETypes:   allowedMIMETypes,
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseInt(value string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseInt64(value string) int64 {
	return parseInt(value)
}

func parseDuration(value string) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 10 * time.Minute
	}
	return d
}

func loadDotEnv(filename string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	for {
		path := filepath.Join(cwd, filename)
		if _, err := os.Stat(path); err == nil {
			return parseDotEnv(path)
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return nil
}

func parseDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
