package services

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"path/filepath"
	"time"

	"github.com/P3chys/entoo2-api/internal/config"
)

type EmailService struct {
	smtpHost      string
	smtpPort      string
	smtpUsername  string
	smtpPassword  string
	fromEmail     string
	fromName      string
	appURL        string
	templatesPath string
	resendAPIKey  string
}

type EmailData struct {
	To           string
	Subject      string
	Body         string
	TemplateData map[string]interface{}
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		smtpHost:      cfg.SMTPHost,
		smtpPort:      cfg.SMTPPort,
		smtpUsername:  cfg.SMTPUsername,
		smtpPassword:  cfg.SMTPPassword,
		fromEmail:     cfg.SMTPFromEmail,
		fromName:      cfg.SMTPFromName,
		appURL:        cfg.AppURL,
		templatesPath: "templates/emails",
		resendAPIKey:  cfg.ResendAPIKey,
	}
}

// SendEmail sends an email using Resend API (if configured) or SMTP
func (s *EmailService) SendEmail(to, subject, body string) error {
	if s.resendAPIKey != "" {
		return s.sendViaResend(to, subject, body)
	}
	return s.sendViaSMTP(to, subject, body)
}

// sendViaResend sends an email using the Resend HTTP API
func (s *EmailService) sendViaResend(to, subject, body string) error {
	payload := map[string]interface{}{
		"from":    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		"to":      []string{to},
		"subject": subject,
		"html":    body,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.resendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email via Resend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Resend API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// sendViaSMTP sends an email using SMTP (STARTTLS or implicit TLS)
func (s *EmailService) sendViaSMTP(to, subject, body string) error {
	from := fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
	msg := []byte(fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", from, to, subject, body))

	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)

	// Development mode without authentication
	if s.smtpUsername == "" && s.smtpPassword == "" {
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, s.smtpHost)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer client.Close()

		if err := client.Mail(s.fromEmail); err != nil {
			return fmt.Errorf("failed to set sender: %w", err)
		}
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient: %w", err)
		}

		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("failed to get data writer: %w", err)
		}
		if _, err = w.Write(msg); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
		if err = w.Close(); err != nil {
			return fmt.Errorf("failed to close data writer: %w", err)
		}

		return client.Quit()
	}

	tlsConfig := &tls.Config{ServerName: s.smtpHost}

	var client *smtp.Client

	if s.smtpPort == "465" {
		// Port 465: implicit TLS (SMTPS) - connect with TLS directly
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		tlsConn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server (TLS): %w", err)
		}
		tlsConn.SetDeadline(time.Now().Add(30 * time.Second))

		client, err = smtp.NewClient(tlsConn, s.smtpHost)
		if err != nil {
			tlsConn.Close()
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
	} else {
		// Port 587: STARTTLS - connect plain, then upgrade
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		conn.SetDeadline(time.Now().Add(30 * time.Second))

		client, err = smtp.NewClient(conn, s.smtpHost)
		if err != nil {
			conn.Close()
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}

		if err := client.StartTLS(tlsConfig); err != nil {
			client.Close()
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}
	defer client.Close()

	// Authenticate
	auth := smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Send
	if err := client.Mail(s.fromEmail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// SendEmailAsync sends an email asynchronously so it doesn't block the HTTP handler
func (s *EmailService) SendEmailAsync(to, subject, body string) {
	go func() {
		if err := s.SendEmail(to, subject, body); err != nil {
			log.Printf("Failed to send email to %s: %v", to, err)
		}
	}()
}

// SendVerificationEmail sends an email verification link to the user
func (s *EmailService) SendVerificationEmail(to, token, language string) error {
	verificationURL := fmt.Sprintf("%s/verify-email/%s", s.appURL, token)

	var subject string
	if language == "cs" {
		subject = "Ověřte svůj e-mail - Entoo2"
	} else {
		subject = "Verify Your Email - Entoo2"
	}

	body, err := s.renderTemplate(fmt.Sprintf("verification_%s.html", language), map[string]interface{}{
		"VerificationURL": verificationURL,
		"AppURL":          s.appURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.SendEmail(to, subject, body)
}

// SendPasswordResetEmail sends a password reset link to the user
func (s *EmailService) SendPasswordResetEmail(to, token, language string) error {
	resetURL := fmt.Sprintf("%s/reset-password/%s", s.appURL, token)

	var subject string
	if language == "cs" {
		subject = "Obnovení hesla - Entoo2"
	} else {
		subject = "Password Reset - Entoo2"
	}

	body, err := s.renderTemplate(fmt.Sprintf("reset_%s.html", language), map[string]interface{}{
		"ResetURL": resetURL,
		"AppURL":   s.appURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.SendEmail(to, subject, body)
}

// renderTemplate loads and renders an email template
func (s *EmailService) renderTemplate(templateName string, data map[string]interface{}) (string, error) {
	templatePath := filepath.Join(s.templatesPath, templateName)

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}
