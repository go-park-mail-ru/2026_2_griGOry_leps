package mail

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

const resetSubject = "Восстановление пароля на Go&Get"

const resetText = `Здравствуйте!

Мы получили запрос на сброс пароля для вашего аккаунта на Go&Get. Чтобы задать новый пароль, перейдите по ссылке:

%s

Ссылка действует 30 минут. Если вы не запрашивали сброс, просто проигнорируйте это письмо.
`

var resetHTML = template.Must(template.New("reset").Parse(`<!doctype html>
<html lang="ru">
<body style="margin:0;padding:24px;background:#f5f5f7;font-family:Arial,Helvetica,sans-serif;color:#1d1d1f">
  <div style="max-width:480px;margin:0 auto;padding:32px;background:#ffffff;border-radius:16px">
    <h1 style="margin:0 0 16px;font-size:22px">Восстановление пароля</h1>
    <p style="margin:0 0 24px;font-size:15px;line-height:1.5">Мы получили запрос на сброс пароля для вашего аккаунта на Go&amp;Get. Нажмите на кнопку, чтобы задать новый пароль.</p>
    <a href="{{.}}" style="display:inline-block;padding:14px 28px;background:#2f6bff;color:#ffffff;text-decoration:none;border-radius:10px;font-size:15px">Задать новый пароль</a>
    <p style="margin:24px 0 0;font-size:13px;line-height:1.5;color:#6e6e73">Ссылка действует 30 минут. Если вы не запрашивали сброс, просто проигнорируйте это письмо.</p>
  </div>
</body>
</html>
`))

const verifySubject = "Подтверждение почты на Go&Get"

const verifyText = `Здравствуйте!

Подтвердите почту для аккаунта на Go&Get. Для этого перейдите по ссылке:

%s

Ссылка действует 30 минут. Если вы не регистрировались на Go&Get и не меняли там почту, просто проигнорируйте это письмо.
`

var verifyHTML = template.Must(template.New("verify").Parse(`<!doctype html>
<html lang="ru">
<body style="margin:0;padding:24px;background:#f5f5f7;font-family:Arial,Helvetica,sans-serif;color:#1d1d1f">
  <div style="max-width:480px;margin:0 auto;padding:32px;background:#ffffff;border-radius:16px">
    <h1 style="margin:0 0 16px;font-size:22px">Подтвердите почту</h1>
    <p style="margin:0 0 24px;font-size:15px;line-height:1.5">Нажмите на кнопку, чтобы подтвердить почту для аккаунта на Go&amp;Get.</p>
    <a href="{{.}}" style="display:inline-block;padding:14px 28px;background:#2f6bff;color:#ffffff;text-decoration:none;border-radius:10px;font-size:15px">Подтвердить почту</a>
    <p style="margin:24px 0 0;font-size:13px;line-height:1.5;color:#6e6e73">Ссылка действует 30 минут. Если вы не регистрировались на Go&amp;Get и не меняли там почту, просто проигнорируйте это письмо.</p>
  </div>
</body>
</html>
`))

func emailVerificationMessage(from *mail.Address, to, link string) ([]byte, error) {
	var html bytes.Buffer
	if err := verifyHTML.Execute(&html, link); err != nil {
		return nil, fmt.Errorf("render verification mail: %w", err)
	}
	return buildMessage(from, to, verifySubject, fmt.Sprintf(verifyText, link), html.String())
}

func passwordResetMessage(from *mail.Address, to, link string) ([]byte, error) {
	var html bytes.Buffer
	if err := resetHTML.Execute(&html, link); err != nil {
		return nil, fmt.Errorf("render reset mail: %w", err)
	}
	return buildMessage(from, to, resetSubject, fmt.Sprintf(resetText, link), html.String())
}

func buildMessage(from *mail.Address, to, subject, text, html string) ([]byte, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, part := range []struct{ contentType, content string }{
		{"text/plain; charset=UTF-8", text},
		{"text/html; charset=UTF-8", html},
	} {
		w, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.contentType},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		qp := quotedprintable.NewWriter(w)
		if _, err := qp.Write([]byte(part.content)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	var msg bytes.Buffer
	headers := [][2]string{
		{"From", from.String()},
		{"To", to},
		{"Subject", mime.QEncoding.Encode("UTF-8", subject)},
		{"Date", time.Now().Format(time.RFC1123Z)},
		{"Message-ID", messageID(from.Address)},
		{"MIME-Version", "1.0"},
		{"Content-Type", "multipart/alternative; boundary=" + mw.Boundary()},
	}
	for _, h := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", h[0], h[1])
	}
	msg.WriteString("\r\n")
	msg.Write(body.Bytes())
	return msg.Bytes(), nil
}

func messageID(from string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	domain := from[strings.LastIndex(from, "@")+1:]
	return "<" + hex.EncodeToString(b) + "@" + domain + ">"
}
