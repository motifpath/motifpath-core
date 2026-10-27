package email

import (
	"context"
	"errors"
	"io"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motifpath/core-domain/internal/domain"
)

// scriptedSMTP is an SMTP server that answers each command with the reply
// it was given, the way a real server would refuse or accept a message at
// each step. It offers no STARTTLS and no AUTH.
type scriptedSMTP struct {
	mailReply string
	rcptReply string
	dataReply string
	// dropOnQuit closes the connection without answering QUIT, as a server
	// that went away after accepting the message would.
	dropOnQuit bool
}

func (s scriptedSMTP) start(t *testing.T) (host, port string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closed := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			closed <- nil
			return
		}
		s.serve(textproto.NewConn(conn))
		closed <- conn.Close()
	}()
	t.Cleanup(func() {
		require.NoError(t, listener.Close())
		require.NoError(t, <-closed)
	})
	host, port, err = net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return host, port
}

func (s scriptedSMTP) serve(conn *textproto.Conn) {
	reply := func(line string) bool { return conn.PrintfLine("%s", line) == nil }
	if !reply("220 scripted ESMTP") {
		return
	}
	for {
		line, err := conn.ReadLine()
		if err != nil {
			return
		}
		command := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		var ok bool
		switch command {
		case "EHLO", "HELO":
			ok = reply("250 scripted")
		case "MAIL":
			ok = reply(s.mailReply)
		case "RCPT":
			ok = reply(s.rcptReply)
		case "DATA":
			ok = reply("354 go ahead") && s.readMessage(conn) && reply(s.dataReply)
		case "QUIT":
			if !s.dropOnQuit {
				reply("221 bye")
			}
			return
		default:
			ok = reply("250 ok")
		}
		if !ok {
			return
		}
	}
}

func (s scriptedSMTP) readMessage(conn *textproto.Conn) bool {
	_, err := io.Copy(io.Discard, conn.DotReader())
	return err == nil
}

func accepting() scriptedSMTP {
	return scriptedSMTP{mailReply: "250 ok", rcptReply: "250 ok", dataReply: "250 queued"}
}

func TestSMTPSender_SendOutcome(t *testing.T) {
	tests := []struct {
		name         string
		server       func() scriptedSMTP
		wantErr      bool
		wantRejected bool
	}{
		{name: "delivered", server: accepting},
		{
			name:   "delivered when the server goes away after accepting the message",
			server: func() scriptedSMTP { s := accepting(); s.dropOnQuit = true; return s },
		},
		{
			name:    "refused for good at the recipient",
			server:  func() scriptedSMTP { s := accepting(); s.rcptReply = "550 mailbox unavailable"; return s },
			wantErr: true, wantRejected: true,
		},
		{
			name:    "refused for good once the message was sent",
			server:  func() scriptedSMTP { s := accepting(); s.dataReply = "554 Access denied"; return s },
			wantErr: true, wantRejected: true,
		},
		{
			name:    "refused for now at the recipient",
			server:  func() scriptedSMTP { s := accepting(); s.rcptReply = "450 mailbox busy"; return s },
			wantErr: true,
		},
		{
			name:    "refused for now once the message was sent",
			server:  func() scriptedSMTP { s := accepting(); s.dataReply = "451 try again later"; return s },
			wantErr: true,
		},
		{
			// The sender address is the platform's own setup, not this
			// email's: fixing it should let every email go out.
			name:    "the sender address is refused",
			server:  func() scriptedSMTP { s := accepting(); s.mailReply = "553 sender not allowed"; return s },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := tt.server().start(t)
			sender := NewSMTPSender(SMTPConfig{Host: host, Port: port, From: "noreply@mail.motifpath.com"}, time.Now)

			err := sender.Send(context.Background(), domain.Email{To: "alice@example.com", Subject: "Hi", Text: "Hi", HTML: "<p>Hi</p>"})

			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tt.wantRejected, errors.Is(err, domain.ErrEmailRejected))
		})
	}
}
