package web

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type PasswordResetMailer interface {
	SendPasswordReset(ctx context.Context, to, resetURL string) error
}

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

type smtpPasswordResetMailer struct {
	config SMTPConfig
}

func allowPlaintextSMTPHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func NewSMTPPasswordResetMailer(config SMTPConfig) PasswordResetMailer {
	config.Host = strings.TrimSpace(config.Host)
	config.Port = strings.TrimSpace(config.Port)
	config.From = strings.TrimSpace(config.From)
	if config.Host == "" || config.Port == "" || config.From == "" {
		return nil
	}
	return &smtpPasswordResetMailer{config: config}
}

func (m *smtpPasswordResetMailer) SendPasswordReset(ctx context.Context, to, resetURL string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	toAddress, err := mail.ParseAddress(strings.TrimSpace(to))
	if err != nil {
		return fmt.Errorf("parse recipient: %w", err)
	}
	fromAddress, err := mail.ParseAddress(m.config.From)
	if err != nil {
		return fmt.Errorf("parse sender: %w", err)
	}

	var auth smtp.Auth
	if m.config.Username != "" {
		auth = smtp.PlainAuth("", m.config.Username, m.config.Password, m.config.Host)
	}

	subject := "Reset your ManaTomb password"
	body := "A password reset was requested for your ManaTomb account.\r\n\r\n" +
		"Use this link within one hour:\r\n" + resetURL + "\r\n\r\n" +
		"If you did not request this, you can ignore this email.\r\n"
	message := []byte(
		"From: " + m.config.From + "\r\n" +
			"To: " + toAddress.String() + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" + body,
	)

	address := net.JoinHostPort(m.config.Host, m.config.Port)
	connection, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer connection.Close()

	deadline := time.Now().Add(15 * time.Second)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return err
	}

	client, err := smtp.NewClient(connection, m.config.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: m.config.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if !allowPlaintextSMTPHost(m.config.Host) {
		return fmt.Errorf("smtp server %q does not advertise STARTTLS", m.config.Host)
	}
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(fromAddress.Address); err != nil {
		return err
	}
	if err := client.Rcpt(toAddress.Address); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
