//go:build integration

package email

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/motifpath/core-domain/internal/domain"
)

// mailpit is a Mailpit container: an SMTP server that keeps every message
// it receives and shows it through an HTTP API.
type mailpit struct {
	smtpHost string
	smtpPort string
	apiURL   string
}

func startMailpit(t *testing.T) mailpit {
	t.Helper()
	ctx := context.Background()
	container, err := testcontainers.Run(ctx, "axllent/mailpit:v1.21.8",
		testcontainers.WithExposedPorts("1025/tcp", "8025/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("1025/tcp"), wait.ForListeningPort("8025/tcp")),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	smtpPort, err := container.MappedPort(ctx, "1025/tcp")
	require.NoError(t, err)
	apiPort, err := container.MappedPort(ctx, "8025/tcp")
	require.NoError(t, err)
	return mailpit{smtpHost: host, smtpPort: smtpPort.Port(), apiURL: fmt.Sprintf("http://%s:%s/api/v1", host, apiPort.Port())}
}

type mailpitAddress struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

type mailpitMessage struct {
	ID      string           `json:"ID"`
	From    mailpitAddress   `json:"From"`
	To      []mailpitAddress `json:"To"`
	Subject string           `json:"Subject"`
	Text    string           `json:"Text"`
	HTML    string           `json:"HTML"`
}

func (m mailpit) getJSON(t *testing.T, path string, into any) {
	t.Helper()
	resp, err := http.Get(m.apiURL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(into))
}

// onlyMessage returns the one message Mailpit holds, with its headers.
func (m mailpit) onlyMessage(t *testing.T) (mailpitMessage, map[string][]string) {
	t.Helper()
	var list struct {
		Messages []struct {
			ID string `json:"ID"`
		} `json:"messages"`
	}
	m.getJSON(t, "/messages", &list)
	require.Len(t, list.Messages, 1)
	var message mailpitMessage
	m.getJSON(t, "/message/"+list.Messages[0].ID, &message)
	headers := map[string][]string{}
	m.getJSON(t, "/message/"+list.Messages[0].ID+"/headers", &headers)
	return message, headers
}

func TestSMTPSender_DeliversAStudentEmail(t *testing.T) {
	server := startMailpit(t)
	sender := NewSMTPSender(SMTPConfig{Host: server.smtpHost, Port: server.smtpPort, From: "noreply@mail.staging.motifpath.com"}, func() time.Time {
		return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	})

	err := sender.Send(context.Background(), domain.Email{
		To:                     "alice@example.com",
		Subject:                "Que tal alguns minutos de prática hoje?",
		Text:                   "Oi, Alice,\n\nJá faz 3 dias.\n",
		HTML:                   "<p>Oi, Alice,</p><p>Já faz 3 dias.</p>",
		OneClickUnsubscribeURL: "https://api.staging.motifpath.com/email-unsubscribes?token=abc.reminders.sig",
	})

	require.NoError(t, err)
	message, headers := server.onlyMessage(t)
	assert.Equal(t, "Que tal alguns minutos de prática hoje?", message.Subject, "a subject with accents arrives intact")
	assert.Equal(t, "noreply@mail.staging.motifpath.com", message.From.Address)
	assert.Equal(t, "MotifPath", message.From.Name)
	require.Len(t, message.To, 1)
	assert.Equal(t, "alice@example.com", message.To[0].Address)
	assert.Contains(t, message.Text, "Já faz 3 dias.")
	assert.Contains(t, message.HTML, "<p>Já faz 3 dias.</p>")
	assert.Equal(t, []string{"<https://api.staging.motifpath.com/email-unsubscribes?token=abc.reminders.sig>"}, headers["List-Unsubscribe"])
	assert.Equal(t, []string{"List-Unsubscribe=One-Click"}, headers["List-Unsubscribe-Post"])
	require.Len(t, headers["Message-Id"], 1)
	assert.True(t, strings.HasSuffix(headers["Message-Id"][0], "@mail.staging.motifpath.com>"))
}

func TestSMTPSender_TeamEmailHasNoUnsubscribe(t *testing.T) {
	server := startMailpit(t)
	sender := NewSMTPSender(SMTPConfig{Host: server.smtpHost, Port: server.smtpPort, From: "noreply@mail.motifpath.com"}, time.Now)

	err := sender.Send(context.Background(), domain.Email{To: "concierge@motifpath.com", Subject: "Students, week of Oct 26", Text: "rows", HTML: "<p>rows</p>"})

	require.NoError(t, err)
	_, headers := server.onlyMessage(t)
	assert.NotContains(t, headers, "List-Unsubscribe")
	assert.NotContains(t, headers, "List-Unsubscribe-Post")
}

func TestSMTPSender_RefusesHeaderInjection(t *testing.T) {
	sender := NewSMTPSender(SMTPConfig{Host: "127.0.0.1", Port: "1", From: "noreply@mail.motifpath.com"}, time.Now)

	err := sender.Send(context.Background(), domain.Email{To: "alice@example.com\r\nBcc: everyone@example.com", Subject: "Hi", Text: "x", HTML: "x"})

	require.Error(t, err)
}

func TestSMTPSender_ReportsAnUnreachableServer(t *testing.T) {
	sender := NewSMTPSender(SMTPConfig{Host: "127.0.0.1", Port: "1", From: "noreply@mail.motifpath.com"}, time.Now)

	err := sender.Send(context.Background(), domain.Email{To: "alice@example.com", Subject: "Hi", Text: "x", HTML: "x"})

	require.Error(t, err)
}
