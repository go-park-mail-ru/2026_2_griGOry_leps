package mail

import (
	"bufio"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func readParts(t *testing.T, raw []byte) (*mail.Message, map[string]string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	require.NoError(t, err)

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/alternative", mediaType)

	parts := make(map[string]string)
	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		p, err := mr.NextRawPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		require.Equal(t, "quoted-printable", p.Header.Get("Content-Transfer-Encoding"))
		body, err := io.ReadAll(quotedprintable.NewReader(p))
		require.NoError(t, err)
		ct, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		require.NoError(t, err)
		parts[ct] = string(body)
	}
	return msg, parts
}

func TestPasswordResetMessage(t *testing.T) {
	from, err := mail.ParseAddress("Go&Get <noreply@goget.pw>")
	require.NoError(t, err)
	link := "http://goget.pw:8001/reset-password?token=abc&x=1"

	raw, err := passwordResetMessage(from, "anna@goget.pw", link)
	require.NoError(t, err)

	msg, parts := readParts(t, raw)
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	require.NoError(t, err)
	require.Equal(t, resetSubject, subject)
	require.Equal(t, "anna@goget.pw", msg.Header.Get("To"))
	require.Contains(t, msg.Header.Get("Message-ID"), "@goget.pw>")
	require.Contains(t, parts["text/plain"], link)
	require.Contains(t, parts["text/html"], `href="http://goget.pw:8001/reset-password?token=abc&amp;x=1"`)
}

func TestEmailVerificationMessage(t *testing.T) {
	from, err := mail.ParseAddress("Go&Get <noreply@goget.pw>")
	require.NoError(t, err)
	link := "http://goget.pw:8001/verify-email?token=abc&x=1"

	raw, err := emailVerificationMessage(from, "anna@goget.pw", link)
	require.NoError(t, err)

	msg, parts := readParts(t, raw)
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	require.NoError(t, err)
	require.Equal(t, verifySubject, subject)
	require.Equal(t, "anna@goget.pw", msg.Header.Get("To"))
	require.Contains(t, parts["text/plain"], link)
	require.Contains(t, parts["text/html"], `href="http://goget.pw:8001/verify-email?token=abc&amp;x=1"`)
}

type fakeSMTP struct {
	addr     string
	received chan string
}

func startFakeSMTP(t *testing.T) fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	srv := fakeSMTP{addr: ln.Addr().String(), received: make(chan string, 1)}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		r := bufio.NewReader(conn)
		reply := func(s string) { _, _ = io.WriteString(conn, s+"\r\n") }

		var dialog strings.Builder
		reply("220 fake ESMTP")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			dialog.WriteString(line)
			switch cmd := strings.ToUpper(strings.Fields(line)[0]); cmd {
			case "EHLO":
				reply("250-fake")
				reply("250 AUTH PLAIN")
			case "AUTH":
				reply("235 ok")
			case "MAIL", "RCPT":
				reply("250 ok")
			case "DATA":
				reply("354 go on")
				for {
					l, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if l == ".\r\n" {
						break
					}
					dialog.WriteString(l)
				}
				reply("250 queued")
			case "QUIT":
				reply("221 bye")
				srv.received <- dialog.String()
				return
			default:
				reply("502 unknown")
			}
		}
	}()
	return srv
}

func TestSMTP_SendPasswordReset(t *testing.T) {
	srv := startFakeSMTP(t)
	_, port, err := net.SplitHostPort(srv.addr)
	require.NoError(t, err)

	m, err := NewSMTP(SMTPConfig{Host: "localhost", Port: port, User: "bot", Password: "secret", From: "Go&Get <noreply@goget.pw>"})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, m.SendPasswordReset(ctx, "anna@goget.pw", "http://goget.pw:8001/reset-password?token=abc"))

	dialog := <-srv.received
	require.Contains(t, dialog, "AUTH PLAIN")
	require.Contains(t, dialog, "MAIL FROM:<noreply@goget.pw>")
	require.Contains(t, dialog, "RCPT TO:<anna@goget.pw>")
	require.Contains(t, dialog, "To: anna@goget.pw")
}

func TestSMTP_ConnectError(t *testing.T) {
	m, err := NewSMTP(SMTPConfig{Host: "127.0.0.1", Port: "1", From: "noreply@goget.pw"})
	require.NoError(t, err)

	err = m.SendPasswordReset(context.Background(), "anna@goget.pw", "http://x")

	require.ErrorContains(t, err, "connect to smtp")
}

func TestNewSMTP_BadFrom(t *testing.T) {
	_, err := NewSMTP(SMTPConfig{From: "not an address"})

	require.Error(t, err)
}
