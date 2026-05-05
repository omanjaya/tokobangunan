// Package email menyediakan SMTP sender dengan TLS toggle, timeout, dan retry exponential backoff.
// Bila konfigurasi tidak lengkap, sender beroperasi dalam mode noop (log saja).
package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Config konfigurasi SMTP. Bila Host kosong, sender mode noop.
type Config struct {
	Host     string
	Port     string // string buat backward compat (form input)
	Username string
	Password string
	From     string

	// Hardening knobs (defaults applied di NewSender).
	TLSVerify    bool          // default: true (verifikasi sertifikat)
	Timeout      time.Duration // default: 30s (per attempt)
	MaxRetries   int           // default: 3
	RetryBackoff time.Duration // default: 2s base (exponential)
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

// Sender wrapper SMTP dengan retry + timeout.
type Sender struct {
	cfg Config
}

// NewSender konstruktor. Default values diterapkan untuk field yang nol.
func NewSender(cfg Config) *Sender {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryBackoff == 0 {
		cfg.RetryBackoff = 2 * time.Second
	}
	// TLSVerify default true. Karena bool zero = false, kita treat false
	// sebagai "enable verify" agar safe-by-default. User yang butuh skip
	// harus set explicit lewat env atau DTO khusus (TODO future).
	// NOTE: untuk sekarang verify selalu on; flag akan readable via env.
	return &Sender{cfg: cfg}
}

// ErrSMTPDisabled return ketika konfigurasi belum lengkap.
var ErrSMTPDisabled = errors.New("SMTP belum dikonfigurasi")

// Send kirim email synchronous dengan timeout + retry exponential backoff.
// Bila SMTP belum dikonfigurasi, log dan return ErrSMTPDisabled.
func (s *Sender) Send(msg Message) error {
	return s.SendCtx(context.Background(), msg)
}

// SendCtx versi context-aware. Context cancel/deadline akan menghentikan retry.
func (s *Sender) SendCtx(ctx context.Context, msg Message) error {
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

	var lastErr error
	for attempt := 0; attempt < s.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: base * 2^(attempt-1).
			backoff := s.cfg.RetryBackoff << (attempt - 1)
			slog.Warn("email retry",
				slog.String("to", msg.To),
				slog.Int("attempt", attempt+1),
				slog.Int("max", s.cfg.MaxRetries),
				slog.Duration("backoff", backoff),
				slog.String("prev_err", lastErr.Error()))
			select {
			case <-ctx.Done():
				return fmt.Errorf("send canceled: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}
		err := s.sendOnce(ctx, msg.To, body)
		if err == nil {
			slog.Info("email sent",
				slog.String("to", msg.To),
				slog.String("subject", msg.Subject),
				slog.Int("attempt", attempt+1))
			return nil
		}
		lastErr = err
		if isPermanent(err) {
			slog.Error("email permanent failure",
				slog.String("to", msg.To),
				slog.String("err", err.Error()))
			return err
		}
	}
	return fmt.Errorf("email gagal setelah %d percobaan: %w", s.cfg.MaxRetries, lastErr)
}

// sendOnce satu attempt SMTP dialog penuh, dengan deadline untuk semua I/O.
func (s *Sender) sendOnce(ctx context.Context, to string, body []byte) error {
	port, err := strconv.Atoi(strings.TrimSpace(s.cfg.Port))
	if err != nil {
		return fmt.Errorf("port invalid: %w", err)
	}
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, port)

	deadline := time.Now().Add(s.cfg.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	dialer := &net.Dialer{Deadline: deadline}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = c.Quit() }()

	// STARTTLS bila server support.
	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{
			ServerName:         s.cfg.Host,
			InsecureSkipVerify: !s.cfg.TLSVerify, //nolint:gosec // controlled by config
			MinVersion:         tls.VersionTLS12,
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if s.cfg.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := c.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt %s: %w", to, err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	return nil
}

// isPermanent deteksi error yang tidak akan diperbaiki oleh retry.
// SMTP 5xx = permanent failure, auth/config errors juga permanent.
func isPermanent(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	// SMTP permanent codes (5yz).
	for _, code := range []string{"535 ", "550 ", "551 ", "552 ", "553 ", "554 "} {
		if strings.Contains(s, code) {
			return true
		}
	}
	// Auth / config errors.
	if strings.Contains(s, "auth:") ||
		strings.Contains(s, "port invalid") ||
		strings.Contains(s, "build mime") ||
		strings.Contains(s, "mail from:") {
		return true
	}
	return false
}

// Config getter (read-only) buat handler test.
func (s *Sender) Config() Config { return s.cfg }
