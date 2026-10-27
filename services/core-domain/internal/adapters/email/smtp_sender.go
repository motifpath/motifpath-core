package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/motifpath/core-domain/internal/domain"
)

// fromName is the display name every engagement email is sent under.
const fromName = "MotifPath"

// sendTimeout bounds one delivery, from dialling to the server's reply, so
// a server that hangs can't stall the mailer.
const sendTimeout = 30 * time.Second

// SMTPConfig says where and as whom email is sent. Without a Username the
// server is used without authentication, as the local Mailpit is. Port
// defaults to 587, where the server is expected to offer STARTTLS.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// SMTPSender delivers email over SMTP: Amazon SES in deployed environments,
// Mailpit locally. It upgrades the connection with STARTTLS whenever the
// server offers it, and authenticates only over an encrypted connection.
type SMTPSender struct {
	cfg SMTPConfig
	now func() time.Time
}

func NewSMTPSender(cfg SMTPConfig, now func() time.Time) *SMTPSender {
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	return &SMTPSender{cfg: cfg, now: now}
}

func (s *SMTPSender) Send(ctx context.Context, email domain.Email) error {
	message, err := s.message(email)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.cfg.Host, s.cfg.Port))
	if err != nil {
		return fmt.Errorf("smtp: dial %s: %w", s.cfg.Host, err)
	}
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return errors.Join(err, conn.Close())
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return errors.Join(fmt.Errorf("smtp: greeting: %w", err), conn.Close())
	}
	if err := s.deliver(client, email.To, message); err != nil {
		return errors.Join(err, client.Close())
	}
	return client.Quit()
}

func (s *SMTPSender) deliver(client *smtp.Client, to string, message []byte) error {
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp: starttls: %w", err)
		}
	}
	if s.cfg.Username != "" {
		// PlainAuth itself refuses to send the password over a connection
		// that isn't encrypted, unless the server is on localhost.
		if err := client.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("smtp: auth: %w", err)
		}
	}
	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp: rcpt to: %w", err)
	}
	body, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}
	if _, err := body.Write(message); err != nil {
		return errors.Join(fmt.Errorf("smtp: write: %w", err), body.Close())
	}
	return body.Close()
}

// message builds the MIME message: headers, then a multipart/alternative
// body with the plain-text part first and the HTML part last, as clients
// prefer the last part they can show. Header values are refused when they
// hold a line break, which would let a value add headers of its own.
func (s *SMTPSender) message(email domain.Email) ([]byte, error) {
	for _, value := range []string{email.To, email.Subject, email.OneClickUnsubscribeURL, s.cfg.From} {
		if strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("smtp: a header value holds a line break")
		}
	}
	if _, err := mail.ParseAddress(email.To); err != nil {
		return nil, fmt.Errorf("smtp: recipient: %w", err)
	}

	var buf bytes.Buffer
	parts := multipart.NewWriter(&buf)
	headers := []string{
		"From: " + (&mail.Address{Name: fromName, Address: s.cfg.From}).String(),
		"To: " + (&mail.Address{Address: email.To}).String(),
		"Subject: " + mime.QEncoding.Encode("utf-8", email.Subject),
		"Date: " + s.now().Format(time.RFC1123Z),
		"Message-ID: " + s.messageID(),
		"MIME-Version: 1.0",
	}
	if email.OneClickUnsubscribeURL != "" {
		headers = append(headers,
			"List-Unsubscribe: <"+email.OneClickUnsubscribeURL+">",
			"List-Unsubscribe-Post: List-Unsubscribe=One-Click",
		)
	}
	headers = append(headers, `Content-Type: multipart/alternative; boundary="`+parts.Boundary()+`"`)
	var message bytes.Buffer
	message.WriteString(strings.Join(headers, "\r\n") + "\r\n\r\n")

	if err := writePart(parts, "text/plain", email.Text); err != nil {
		return nil, err
	}
	if err := writePart(parts, "text/html", email.HTML); err != nil {
		return nil, err
	}
	if err := parts.Close(); err != nil {
		return nil, err
	}
	message.Write(buf.Bytes())
	return message.Bytes(), nil
}

func writePart(parts *multipart.Writer, contentType, content string) error {
	part, err := parts.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {contentType + "; charset=utf-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return err
	}
	encoder := quotedprintable.NewWriter(part)
	if _, err := encoder.Write([]byte(content)); err != nil {
		return err
	}
	return encoder.Close()
}

// messageID is a unique id under the sending domain, which spam filters
// expect a message to carry.
func (s *SMTPSender) messageID() string {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	domainPart := s.cfg.From[strings.LastIndex(s.cfg.From, "@")+1:]
	return "<" + hex.EncodeToString(random) + "@" + domainPart + ">"
}
