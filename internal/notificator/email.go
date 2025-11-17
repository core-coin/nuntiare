package notificator

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
)

const (
	// Email sending retry settings
	MaxEmailRetries    = 3
	EmailRetryBackoff  = 2 * time.Second
	EmailTimeout       = 30 * time.Second
)

type EmailNotificator struct {
	logger *logger.Logger

	SMTPHost            string
	SMTPPort            int
	SMTPAlternativePort int
	SMTPUser            string
	SMTPPassword        string
	SMTPSender          string

	SMTPAuth smtp.Auth

	db models.Repository
}

func NewEmailNotificator(logger *logger.Logger, SMTPHost string, SMTPPort int, SMTPAlternativePort int, SMTPUser string, SMTPPassword string, SMTPSender string, db models.Repository) *EmailNotificator {
	auth := smtp.PlainAuth(
		"",
		SMTPUser,
		SMTPPassword,
		SMTPHost,
	)

	return &EmailNotificator{
		logger:              logger,
		SMTPAuth:            auth,
		db:                  db,
		SMTPHost:            SMTPHost,
		SMTPPort:            SMTPPort,
		SMTPAlternativePort: SMTPAlternativePort,
		SMTPUser:            SMTPUser,
		SMTPPassword:        SMTPPassword,
		SMTPSender:          SMTPSender,
	}
}

func (e *EmailNotificator) SendNotification(to, message string) {
	addr := fmt.Sprintf("%s:%s", e.SMTPHost, strconv.Itoa(e.SMTPPort))
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		e.SMTPSender,   // From address
		to,             // To address
		"Notification", // Subject
		message,        // Email body
	)

	// Retry logic for transient failures
	var lastErr error
	for attempt := 0; attempt < MaxEmailRetries; attempt++ {
		if attempt > 0 {
			// Wait before retrying
			time.Sleep(EmailRetryBackoff * time.Duration(attempt))
			e.logger.Debug("Retrying email send", "attempt", attempt+1, "to", to)
		}

		// Send email with timeout
		err := e.sendMailWithTimeout(addr, e.SMTPAuth, e.SMTPSender, []string{to}, []byte(msg))
		if err == nil {
			e.logger.Debug("Email notification sent successfully", "to", to, "attempt", attempt+1)
			return
		}

		lastErr = err
		e.logger.Warn("Failed to send email", "to", to, "attempt", attempt+1, "error", err)
	}

	e.logger.Error("Failed to send email notification after retries", "to", to, "attempts", MaxEmailRetries, "error", lastErr)
}

// sendMailWithTimeout sends an email with a timeout and TLS support
func (e *EmailNotificator) sendMailWithTimeout(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// Create a dialer with timeout
	dialer := &net.Dialer{
		Timeout: EmailTimeout,
	}

	// Connect with timeout
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Set deadline for the entire operation
	if err := conn.SetDeadline(time.Now().Add(EmailTimeout)); err != nil {
		return fmt.Errorf("failed to set connection deadline: %w", err)
	}

	// Create SMTP client
	client, err := smtp.NewClient(conn, e.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Start TLS if the server supports it (STARTTLS for port 587)
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: e.SMTPHost,
			MinVersion: tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Authenticate if auth is provided
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// Send message body
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to open data writer: %w", err)
	}

	if _, err := writer.Write(msg); err != nil {
		writer.Close()
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	// Send QUIT command
	return client.Quit()
}
