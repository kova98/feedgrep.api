package handlers

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"text/template"
)

type EmailHandler struct {
	logger   *slog.Logger
	smtpHost string
	smtpPort string
	from     string
	password string
}

func NewEmailHandler(logger *slog.Logger, smtpHost, smtpPort, from, password string) *EmailHandler {
	return &EmailHandler{
		logger:   logger,
		smtpHost: smtpHost,
		smtpPort: smtpPort,
		from:     from,
		password: password,
	}
}

func (h *EmailHandler) SendEmail(to, keyword, subreddit, author, title, body, url string, isComment bool) error {
	matchType := "Post"
	if isComment {
		matchType = "Comment"
	}

	subject := fmt.Sprintf("'%s' mentioned in r/%s", keyword, subreddit)

	// Truncate body for email
	emailBody := body
	if len(emailBody) > 500 {
		emailBody = emailBody[:500] + "..."
	}
	emailBody = strings.ReplaceAll(emailBody, "\n", "<br>")

	if title != "" {
		title = fmt.Sprintf("<p><strong>%s</strong></p>", title)
	}

	tmpl := template.Must(template.New("match-email").Parse(`From: feedgrep <{{.From}}>
To: {{.To}}
Subject: {{.Subject}}
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8

<html>
<body style="margin:0; background:#ffffff; font-family:Arial, Helvetica, sans-serif; color:#202124;">
<div style="font-size:14px; line-height:1.5;">
  <div style="color:#5f6368; font-size:13px; margin-bottom:10px;"><strong>r/{{.Subreddit}}</strong> · u/{{.Author}} · {{.MatchType}}</div>
  {{.Title}}
  <p>{{.Body}}</p>
  <p><a href="{{.URL}}" style="color:#1a73e8; text-decoration:none;">View on Reddit</a></p>
</div>
</body>
</html>
`))

	data := struct {
		From      string
		To        string
		Subject   string
		Subreddit string
		Author    string
		MatchType string
		Title     string
		Body      string
		URL       string
	}{
		From:      h.from,
		To:        to,
		Subject:   subject,
		Subreddit: subreddit,
		Author:    author,
		MatchType: matchType,
		Title:     title,
		Body:      emailBody,
		URL:       url,
	}

	var message bytes.Buffer
	if err := tmpl.Execute(&message, data); err != nil {
		return fmt.Errorf("render email template: %w", err)
	}

	auth := smtp.PlainAuth("", h.from, h.password, h.smtpHost)
	addr := fmt.Sprintf("%s:%s", h.smtpHost, h.smtpPort)
	err := smtp.SendMail(addr, auth, h.from, []string{to}, message.Bytes())
	if err != nil {
		h.logger.Error("Failed to send email", "error", err)
		return err
	}

	h.logger.Info("Email sent", "keyword", keyword, "subreddit", subreddit, "type", matchType)
	return nil
}
