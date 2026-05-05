// Package email menyediakan SMTP sender sederhana dengan dukungan attachment.
// Bila konfigurasi tidak lengkap, sender beroperasi dalam mode noop (log saja).
package email

import (
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
)

// Config konfigurasi SMTP. Bila Host kosong, sender mode noop.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// Enabled cek konfigurasi minimum tersedia.
func (c Config) Enabled() bool {
	return strings.TrimSpace(c.Host) != "" &&
		strings.TrimSpace(c.Port) != "" &&
		strings.TrimSpace(c.From) != ""
}

// Attachment lampiran email.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

// Message satu email yang akan dikirim.
type Message struct {
	To          string
	Subject     string
	BodyHTML    string
	BodyText    string
	Attachments []Attachment
}

// Sender wrapper SMTP.
type Sender struct {
	cfg Config
}

// NewSender konstruktor.
func NewSender(cfg Config) *Sender {
	return &Sender{cfg: cfg}
}

// ErrSMTPDisabled return ketika konfigurasi belum lengkap.
var ErrSMTPDisabled = errors.New("SMTP belum dikonfigurasi")

// Send kirim email. Bila SMTP belum dikonfigurasi, log dan return ErrSMTPDisabled.
func (s *Sender) Send(msg Message) error {
	if !s.cfg.Enabled() {
		slog.Warn("email send skipped: SMTP disabled",
			slog.String("to", msg.To),
			slog.String("subject", msg.Subject))
		return ErrSMTPDisabled
	}
	if strings.TrimSpace(msg.To) == "" {
		return errors.New("email to kosong")
	}

	body, err := buildMIME(s.cfg.From, msg)
	if err != nil {
		return fmt.Errorf("build mime: %w", err)
	}

	addr := s.cfg.Host + ":" + s.cfg.Port
	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{msg.To}, body); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	slog.Info("email sent",
		slog.String("to", msg.To),
		slog.String("subject", msg.Subject))
	return nil
}

// Config getter (read-only) buat handler test.
func (s *Sender) Config() Config { return s.cfg }
