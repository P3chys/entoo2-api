package services

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net"
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
	}
}

// SendEmail sends an email using SMTP with STARTTLS and a connection timeout
func (s *EmailService) SendEmail(to, subject, body string) error {
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

	// Production mode: connect with timeout, then STARTTLS
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	client, err := smtp.NewClient(conn, s.smtpHost)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// STARTTLS
	tlsConfig := &tls.Config{ServerName: s.smtpHost}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

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
