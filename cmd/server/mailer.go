package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"

	"github.com/kartikeyyadav/spendbuddy/pkg/config"
)

type smtpMailer struct {
	cfg config.SMTPConfig
}

func newSMTPMailer(cfg config.SMTPConfig) *smtpMailer {
	return &smtpMailer{cfg: cfg}
}

func (m *smtpMailer) SendOTP(_ context.Context, to, code string) error {
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	auth := smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)

	body := fmt.Sprintf(
		"From: SpendBuddy AI <%s>\r\nTo: %s\r\nSubject: Your OTP Code\r\n\r\nYour one-time code is: %s\r\nIt expires in %d minutes.",
		m.cfg.User, to, code, int(m.cfg.OTPTTL.Minutes()),
	)

	tlsCfg := &tls.Config{ServerName: m.cfg.Host}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Quit()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(m.cfg.User); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = fmt.Fprint(w, body)
	return err
}
