package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
)

type SMTPConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

type SMTP struct {
	cfg  SMTPConfig
	from *mail.Address
}

func NewSMTP(cfg SMTPConfig) (*SMTP, error) {
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("parse MAIL_FROM: %w", err)
	}
	return &SMTP{cfg: cfg, from: from}, nil
}

func (m *SMTP) SendPasswordReset(ctx context.Context, to, link string) error {
	msg, err := passwordResetMessage(m.from, to, link)
	if err != nil {
		return err
	}
	return m.send(ctx, to, msg)
}

func (m *SMTP) SendEmailVerification(ctx context.Context, to, link string) error {
	msg, err := emailVerificationMessage(m.from, to, link)
	if err != nil {
		return err
	}
	return m.send(ctx, to, msg)
}

func (m *SMTP) send(ctx context.Context, to string, msg []byte) error {
	conn, err := m.dial(ctx)
	if err != nil {
		return fmt.Errorf("connect to smtp: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp hello: %w", err)
	}
	defer func() { _ = c.Close() }()

	if ok, _ := c.Extension("STARTTLS"); ok && !m.implicitTLS() {
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if m.cfg.User != "" {
		if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := c.Mail(m.from.Address); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return c.Quit()
}

func (m *SMTP) dial(ctx context.Context) (net.Conn, error) {
	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	if m.implicitTLS() {
		d := &tls.Dialer{Config: &tls.Config{ServerName: m.cfg.Host}}
		return d.DialContext(ctx, "tcp", addr)
	}
	var d net.Dialer
	return d.DialContext(ctx, "tcp", addr)
}

func (m *SMTP) implicitTLS() bool {
	return m.cfg.Port == "465"
}
