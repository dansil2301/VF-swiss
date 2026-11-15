package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vfswiss/vfswiss/internal/config"
	"github.com/vfswiss/vfswiss/internal/middleware"
	"github.com/vfswiss/vfswiss/internal/service"
)

type Handlers struct {
	templates map[string]*template.Template
	svc       *service.Service
	cfg       *config.Config
	logger    *slog.Logger
	limiter   *middleware.RateLimiter
}

type ExampleMedia struct {
	Name    string
	IsVideo bool
	Kind    string
	Poster  string
}

type TemplateData struct {
	Title            string
	Description      string
	CSRFToken        string
	FormErrors       map[string]string
	FormValues       map[string]string
	Message          string
	Examples         []ExampleMedia
	FormStartedToken string
	TurnstileSiteKey string
}

const (
	minimumFormDuration = 5 * time.Second
	maximumFormAge      = 24 * time.Hour
	turnstileVerifyURL  = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
)

var turnstileHTTPClient = &http.Client{Timeout: 5 * time.Second}

func New(templates map[string]*template.Template, cfg *config.Config, svc *service.Service, logger *slog.Logger, limiter *middleware.RateLimiter) *Handlers {
	return &Handlers{templates: templates, cfg: cfg, svc: svc, logger: logger, limiter: limiter}
}

func (h *Handlers) Home(w http.ResponseWriter, r *http.Request) {
	token := h.loadCSRFToken(w, r)
	data := TemplateData{
		Title:            "VF Swiss — ремонт швейцарских часов в Москве",
		Description:      "Бутиковый сервис ремонта и обслуживания швейцарских часов в Москве. Rolex, Omega, Breitling, Cartier, TAG Heuer, Longines, Patek Philippe.",
		CSRFToken:        token,
		FormValues:       map[string]string{},
		FormStartedToken: h.newFormStartedToken(token, time.Now()),
		TurnstileSiteKey: h.cfg.TurnstileSiteKey,
	}
	h.render(w, http.StatusOK, "home", data)
}

func (h *Handlers) SubmitRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.NotFound(w, r)
		return
	}
	if !h.validateCSRF(r) {
		h.errorResponse(w, r, http.StatusBadRequest, "Неверный CSRF токен")
		return
	}

	ip := clientIP(r)
	if !h.limiter.Allow(ip) {
		h.errorResponse(w, r, http.StatusTooManyRequests, "Слишком много запросов. Попробуйте позже.")
		return
	}

	if err := r.ParseMultipartForm(h.cfg.MaxUploadTotal + 1024); err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "Не удалось прочитать форму")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	brand := strings.TrimSpace(r.FormValue("brand"))
	if brand == "" {
		brand = strings.TrimSpace(r.FormValue("brand_custom"))
	}
	if brand == "" {
		brand = strings.TrimSpace(r.FormValue("brand_select"))
	}
	values := map[string]string{
		"name":        strings.TrimSpace(r.FormValue("name")),
		"phone":       strings.TrimSpace(r.FormValue("phone")),
		"email":       strings.TrimSpace(r.FormValue("email")),
		"brand":       brand,
		"description": strings.TrimSpace(r.FormValue("description")),
	}
	honeypot := strings.TrimSpace(r.FormValue("contact_me"))
	if honeypot != "" {
		h.logger.Info("honeypot triggered", "ip", ip)
		http.Redirect(w, r, "/success", http.StatusSeeOther)
		return
	}
	if !h.validFormDuration(r) {
		h.renderFormError(w, r, values, "Пожалуйста, подождите несколько секунд перед отправкой.")
		return
	}
	if h.cfg.TurnstileSecretKey != "" {
		if err := h.verifyTurnstile(r.Context(), r.FormValue("cf-turnstile-response")); err != nil {
			h.logger.Warn("turnstile verification failed", "error", err, "ip", ip)
			h.renderFormError(w, r, values, "Пожалуйста, пройдите проверку безопасности и отправьте заявку ещё раз.")
			return
		}
	}

	files, err := h.parseUploads(r)
	if err != nil {
		h.renderFormError(w, r, values, err.Error())
		return
	}

	input := service.CreateRequestInput{
		Name:        values["name"],
		Phone:       values["phone"],
		Email:       values["email"],
		Brand:       values["brand"],
		Description: values["description"],
		Photos:      files,
		Honeypot:    honeypot,
	}

	_, err = h.svc.CreateRepairRequest(r.Context(), input)
	if err != nil {
		if err == service.ErrHoneypot {
			http.Redirect(w, r, "/success", http.StatusSeeOther)
			return
		}
		if err == service.ErrInvalidName || err == service.ErrInvalidPhone || err == service.ErrInvalidEmail || err == service.ErrInvalidBrand || err == service.ErrInvalidDescription {
			h.renderFormError(w, r, values, "Пожалуйста, проверьте введённые данные.")
			return
		}
		h.logger.Error("request creation failed", "error", err)
		h.errorResponse(w, r, http.StatusInternalServerError, "Не удалось отправить заявку. Попробуйте позже.")
		return
	}

	http.Redirect(w, r, "/success", http.StatusSeeOther)
}

func (h *Handlers) Success(w http.ResponseWriter, r *http.Request) {
	data := TemplateData{
		Title:       "Спасибо! Заявка принята",
		Description: "Мы свяжемся с вами в течение рабочего дня.",
	}
	h.render(w, http.StatusOK, "success", data)
}

func (h *Handlers) Examples(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(filepath.Join("web", "static", "img", "exmaples"))
	if err != nil {
		h.logger.Error("read examples folder", "error", err)
		h.errorResponse(w, r, http.StatusInternalServerError, "Не удалось загрузить примеры работ.")
		return
	}

	items := make([]ExampleMedia, 0, len(files))
	imagesByStem := make(map[string]string)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".webp") {
			base := strings.TrimSuffix(lower, filepath.Ext(lower))
			imagesByStem[base] = "/static/img/exmaples/" + name
		}
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		lower := strings.ToLower(name)
		switch {
		case strings.HasSuffix(lower, ".mp4"), strings.HasSuffix(lower, ".mov"), strings.HasSuffix(lower, ".webm"):
			base := strings.TrimSuffix(lower, filepath.Ext(lower))
			items = append(items, ExampleMedia{Name: name, IsVideo: true, Kind: "video", Poster: imagesByStem[base]})
		case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".webp"):
			items = append(items, ExampleMedia{Name: name, IsVideo: false, Kind: "image"})
		}
	}
	if len(items) == 0 {
		h.errorResponse(w, r, http.StatusInternalServerError, "В папке с примерами нет файлов.")
		return
	}

	data := TemplateData{
		Title:       "Примеры работ — VF Swiss",
		Description: "Галерея выполненных работ по ремонту и обслуживанию швейцарских часов.",
		Examples:    items,
	}
	h.render(w, http.StatusOK, "examples", data)
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := h.svc.HealthCheck(r.Context()); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"error"}`))
		return
	}
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handlers) Robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(`User-agent: *
Allow: /
Sitemap: https://vf-swiss.ru/sitemap.xml
`))
}

func (h *Handlers) Sitemap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
		<loc>https://vf-swiss.ru/</loc>
    <changefreq>daily</changefreq>
  </url>
</urlset>
`))
}

func (h *Handlers) NotFound(w http.ResponseWriter, r *http.Request) {
	h.errorResponse(w, r, http.StatusNotFound, "Страница не найдена")
}

func (h *Handlers) errorResponse(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.WriteHeader(status)
	data := TemplateData{
		Title:   http.StatusText(status),
		Message: message,
	}
	if tmpl, ok := h.templates["error"]; ok {
		if err := tmpl.ExecuteTemplate(w, "error", data); err != nil {
			http.Error(w, message, status)
		}
		return
	}
	http.Error(w, message, status)
}

func (h *Handlers) render(w http.ResponseWriter, status int, name string, data TemplateData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	tmpl, ok := h.templates[name]
	if !ok {
		h.logger.Error("template not found", "template", name)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("template render", "error", err, "template", name)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
	}
}

func (h *Handlers) renderFormError(w http.ResponseWriter, r *http.Request, values map[string]string, message string) {
	token := h.loadCSRFToken(w, r)
	data := TemplateData{
		Title:            "Заполните заявку",
		Description:      "",
		CSRFToken:        token,
		FormErrors:       map[string]string{"form": message},
		FormValues:       values,
		FormStartedToken: h.newFormStartedToken(token, time.Now()),
		TurnstileSiteKey: h.cfg.TurnstileSiteKey,
	}
	h.render(w, http.StatusBadRequest, "home", data)
}

func (h *Handlers) newFormStartedToken(csrfToken string, createdAt time.Time) string {
	timestamp := strconv.FormatInt(createdAt.UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(csrfToken))
	_, _ = mac.Write([]byte(timestamp))
	return timestamp + "." + hex.EncodeToString(mac.Sum(nil))
}

func (h *Handlers) validFormDuration(r *http.Request) bool {
	cookie, err := r.Cookie("csrf_token")
	if err != nil || cookie.Value == "" {
		return false
	}
	timestamp, signature, ok := strings.Cut(r.FormValue("form_started_at"), ".")
	if !ok {
		return false
	}
	createdAtMillis, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	providedSignature, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(cookie.Value))
	_, _ = mac.Write([]byte(timestamp))
	if !hmac.Equal(providedSignature, mac.Sum(nil)) {
		return false
	}
	age := time.Since(time.UnixMilli(createdAtMillis))
	return age >= minimumFormDuration && age <= maximumFormAge
}

func (h *Handlers) verifyTurnstile(ctx context.Context, responseToken string) error {
	if responseToken == "" || len(responseToken) > 2048 {
		return fmt.Errorf("missing or oversized challenge token")
	}
	form := url.Values{
		"secret":   {h.cfg.TurnstileSecretKey},
		"response": {responseToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := turnstileHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("siteverify returned status %d", resp.StatusCode)
	}
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("challenge was not successful")
	}
	return nil
}

func (h *Handlers) parseUploads(r *http.Request) ([]service.FileUpload, error) {
	var uploads []service.FileUpload
	files := r.MultipartForm.File["photos"]
	if len(files) > h.cfg.MaxUploadFiles {
		return nil, fmt.Errorf("Можно загрузить не более %d файлов", h.cfg.MaxUploadFiles)
	}
	totalSize := int64(0)
	for _, fh := range files {
		if fh.Size == 0 {
			continue
		}
		if fh.Size > h.cfg.MaxUploadSize {
			return nil, fmt.Errorf("Файл %s слишком большой", fh.Filename)
		}
		file, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("Не удалось открыть файл")
		}
		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("Не удалось прочитать файл %s", fh.Filename)
		}
		if len(data) == 0 {
			continue
		}
		contentType := http.DetectContentType(data[:min(len(data), 512)])
		if !contains(h.cfg.AllowedMIMETypes, contentType) {
			return nil, fmt.Errorf("Неверный формат файла %s", fh.Filename)
		}
		totalSize += fh.Size
		if totalSize > h.cfg.MaxUploadTotal {
			return nil, fmt.Errorf("Суммарный размер файлов превышает %d MB", h.cfg.MaxUploadTotal/1024/1024)
		}

		uploads = append(uploads, service.FileUpload{
			Data:         data,
			OriginalName: path.Base(fh.Filename),
			MIMEType:     contentType,
			Size:         fh.Size,
		})
	}
	return uploads, nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *Handlers) loadCSRFToken(w http.ResponseWriter, r *http.Request) string {
	if r != nil {
		if cookie, err := r.Cookie("csrf_token"); err == nil && cookie.Value != "" {
			return cookie.Value
		}
	}
	token := randomToken(32)
	cookie := &http.Cookie{
		Name:     "csrf_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600,
	}
	cookie.Secure = r != nil && r.TLS != nil
	http.SetCookie(w, cookie)
	return token
}

func (h *Handlers) validateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie("csrf_token")
	if err != nil {
		return false
	}
	token := r.FormValue("csrf_token")
	return token != "" && token == cookie.Value
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host := r.RemoteAddr
	if colon := strings.LastIndex(host, ":"); colon != -1 {
		return host[:colon]
	}
	return host
}
