package services

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"path/filepath"

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
	To          string
	Subject     string
	Body        string
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

// SendEmail sends an email using SMTP with TLS
func (s *EmailService) SendEmail(to, subject, body string) error {
	// SMTP authentication
	auth := smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)

	// Construct email message
	from := fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
	msg := []byte(fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", from, to, subject, body))

	// Connect to SMTP server with TLS
	addr := fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort)

	// For localhost (development), skip TLS verification
	var tlsConfig *tls.Config
	if s.smtpHost == "localhost" || s.smtpHost == "127.0.0.1" {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         s.smtpHost,
		}
	} else {
		tlsConfig = &tls.Config{
			ServerName: s.smtpHost,
		}
	}

	// Send email
	// Note: For localhost development without auth, fall back to simple SMTP
	if s.smtpUsername == "" && s.smtpPassword == "" {
		// Development mode without authentication
		conn, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		defer conn.Close()

		if err := conn.Mail(s.fromEmail); err != nil {
			return fmt.Errorf("failed to set sender: %w", err)
		}
		if err := conn.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient: %w", err)
		}

		w, err := conn.Data()
		if err != nil {
			return fmt.Errorf("failed to get data writer: %w", err)
		}
		_, err = w.Write(msg)
		if err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
		err = w.Close()
		if err != nil {
			return fmt.Errorf("failed to close data writer: %w", err)
		}

		return conn.Quit()
	}

	// Production mode with STARTTLS authentication
	err := smtp.SendMail(addr, auth, s.fromEmail, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendVerificationEmail sends an email verification link to the user
func (s *EmailService) SendVerificationEmail(to, token, language string) error {
	// Build verification URL
	verificationURL := fmt.Sprintf("%s/verify-email/%s", s.appURL, token)

	// Determine subject based on language
	var subject string
	if language == "cs" {
		subject = "Ověřte svůj e-mail - Entoo2"
	} else {
		subject = "Verify Your Email - Entoo2"
	}

	// Load and render template
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
	// Build reset URL
	resetURL := fmt.Sprintf("%s/reset-password/%s", s.appURL, token)

	// Determine subject based on language
	var subject string
	if language == "cs" {
		subject = "Obnovení hesla - Entoo2"
	} else {
		subject = "Password Reset - Entoo2"
	}

	// Load and render template
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

	// Parse template file
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	// Render template with data
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return buf.String(), nil
}
