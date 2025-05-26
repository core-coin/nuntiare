package notificator

import (
	"fmt"
	"log"
	"net/smtp"
	"strconv"

	"github.com/core-coin/nuntiare/internal/models"
	"github.com/core-coin/nuntiare/pkg/logger"
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
	if err := smtp.SendMail(addr, e.SMTPAuth, e.SMTPSender, []string{to}, []byte(msg)); err != nil {
		log.Fatalf("failed to send email: %v", err)
	}
}
