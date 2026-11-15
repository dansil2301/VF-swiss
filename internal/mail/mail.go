package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vfswiss/vfswiss/internal/config"
	"github.com/vfswiss/vfswiss/internal/models"
)

type Client struct {
	cfg     *config.Config
	enabled bool
}

func New(cfg *config.Config) *Client {
	return &Client{cfg: cfg, enabled: cfg.SMTPHost != "" && cfg.SMTPUser != "" && cfg.SMTPPass != "" && cfg.OwnerEmail != ""}
}

func (c *Client) SendRequestNotification(ctx context.Context, request *models.Request, photos []*models.RequestPhoto) error {
	if !c.enabled {
		return fmt.Errorf("smtp is not configured")
	}

	subject := "Новая заявка на ремонт — VF Swiss"
	if strings.TrimSpace(request.Name) != "" {
		subject = fmt.Sprintf("Новая заявка от %s — VF Swiss", request.Name)
	}
	if strings.TrimSpace(subject) == "" {
		subject = "(без темы)"
	}
	// MIME-encode subject to ensure non-ASCII characters are preserved
	subject = mime.QEncoding.Encode("utf-8", subject)
	boundary := "boundary-" + strings.ReplaceAll(request.CreatedAt.Format(time.RFC3339Nano), ":", "")
	header := map[string]string{
		"From":         c.cfg.MailFrom,
		"To":           c.cfg.OwnerEmail,
		"Reply-To":     request.Email,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "multipart/mixed; boundary=" + boundary,
	}
	var body bytes.Buffer
	for k, v := range header {
		body.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	body.WriteString("\r\n")
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	body.WriteString(c.emailText(request, photos))
	body.WriteString("\r\n")

	for _, photo := range photos {
		if err := c.attachPhoto(&body, boundary, photo); err != nil {
			return err
		}
	}
	body.WriteString("--" + boundary + "--\r\n")

	return c.sendRawMail([]string{c.cfg.OwnerEmail}, body.Bytes())
}

func (c *Client) SendUserConfirmation(ctx context.Context, request *models.Request) error {
	if !c.enabled || request.Email == "" {
		return nil
	}

	subject := "Подтверждение заявки — VF Swiss"
	if strings.TrimSpace(request.Name) != "" {
		subject = fmt.Sprintf("Подтверждение заявки для %s — VF Swiss", request.Name)
	}
	if strings.TrimSpace(subject) == "" {
		subject = "(без темы)"
	}
	// MIME-encode subject to ensure non-ASCII characters are preserved
	subject = mime.QEncoding.Encode("utf-8", subject)
	header := map[string]string{
		"From":         c.cfg.MailFrom,
		"To":           request.Email,
		"Reply-To":     c.cfg.OwnerEmail,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=utf-8",
	}
	var body bytes.Buffer
	for k, v := range header {
		body.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	body.WriteString("\r\n")
	body.WriteString(c.userConfirmationText(request))

	return c.sendRawMail([]string{request.Email}, body.Bytes())
}

func (c *Client) sendRawMail(recipients []string, rawMessage []byte) error {
	addr := fmt.Sprintf("%s:%d", c.cfg.SMTPHost, c.cfg.SMTPPort)
	auth := smtp.PlainAuth("", c.cfg.SMTPUser, c.cfg.SMTPPass, c.cfg.SMTPHost)

	if c.cfg.SMTPPort == 465 {
		tlsConfig := &tls.Config{ServerName: c.cfg.SMTPHost}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return err
		}
		client, err := smtp.NewClient(conn, c.cfg.SMTPHost)
		if err != nil {
			return err
		}
		defer client.Close()
		if err := client.Auth(auth); err != nil {
			return err
		}
		if err := client.Mail(c.cfg.MailFrom); err != nil {
			return err
		}
		for _, recipient := range recipients {
			if err := client.Rcpt(recipient); err != nil {
				return err
			}
		}
		wc, err := client.Data()
		if err != nil {
			return err
		}
		_, err = wc.Write(rawMessage)
		if cerr := wc.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		return client.Quit()
	}

	return smtp.SendMail(addr, auth, c.cfg.MailFrom, recipients, rawMessage)
}

func (c *Client) userConfirmationText(request *models.Request) string {
	builder := strings.Builder{}
	builder.WriteString("Здравствуйте, " + request.Name + "!\n\n")
	builder.WriteString("Спасибо, что оставили заявку в сервисе VF Swiss.\n")
	builder.WriteString("Мы получили ваши данные и свяжемся с вами в течение рабочего дня.\n\n")
	builder.WriteString("Детали заявки:\n")
	builder.WriteString("- Телефон: " + request.Phone + "\n")
	builder.WriteString("- E-mail: " + request.Email + "\n")
	if request.Brand != "" {
		builder.WriteString("- Бренд часов: " + request.Brand + "\n")
	}
	if request.Description != "" {
		builder.WriteString("- Описание: " + request.Description + "\n")
	}
	builder.WriteString("- Дата подачи: " + request.CreatedAt.Format("2006-01-02 15:04:05") + "\n\n")
	builder.WriteString("Если вы не получили это письмо, проверьте папку «Спам».\n")
	builder.WriteString("Если у вас есть вопросы, просто ответьте на это письмо — мы ответим оперативно.\n")
	return builder.String()
}

func (c *Client) emailText(request *models.Request, photos []*models.RequestPhoto) string {
	builder := strings.Builder{}
	builder.WriteString("Новая заявка.\n\n")
	builder.WriteString(fmt.Sprintf("Имя: %s\n", request.Name))
	builder.WriteString(fmt.Sprintf("Телефон: %s\n", request.Phone))
	builder.WriteString(fmt.Sprintf("Бренд: %s\n", request.Brand))
	builder.WriteString(fmt.Sprintf("Описание: %s\n", request.Description))
	builder.WriteString(fmt.Sprintf("Дата: %s\n", request.CreatedAt.Format("2006-01-02 15:04:05")))
	if len(photos) > 0 {
		builder.WriteString("Фото:\n")
		for _, photo := range photos {
			builder.WriteString(fmt.Sprintf("- %s (%s, %d bytes)\n", photo.Filename, photo.MIMEType, photo.Size))
		}
	}
	return builder.String()
}

func (c *Client) attachPhoto(body *bytes.Buffer, boundary string, photo *models.RequestPhoto) error {
	path := filepath.Join(c.cfg.UploadDir, photo.Filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	encoded := encodeBase64(content)
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString(fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", photo.MIMEType, mime.QEncoding.Encode("utf-8", photo.OriginalName)))
	body.WriteString("Content-Transfer-Encoding: base64\r\n")
	body.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", mime.QEncoding.Encode("utf-8", photo.OriginalName)))
	body.WriteString(encoded)
	body.WriteString("\r\n")
	return nil
}

func encodeBase64(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var wrapped strings.Builder
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		wrapped.WriteString(encoded[i:end])
		wrapped.WriteString("\r\n")
	}
	return wrapped.String()
}
