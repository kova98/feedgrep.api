package handlers

import (
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
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

	subject := fmt.Sprintf("feedgrep: '%s' mentioned in r/%s", keyword, subreddit)

	// Truncate body for email
	emailBody := body
	if len(emailBody) > 500 {
		emailBody = emailBody[:500] + "..."
	}
	emailBody = strings.ReplaceAll(emailBody, "\n", "<br>")

	if title != "" {
		title = fmt.Sprintf("<p><strong>Title:</strong> %s</p>", title)
	}

	message := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8

<html>
<body>
<h2>🎯 Keyword Match Found</h2>
<p><strong>Type:</strong> %s</p>
<p><strong>Keyword:</strong> %s</p>
<p><strong>Subreddit:</strong> r/%s</p>
<p><strong>Author:</strong> u/%s</p>
%s
<p><strong>Content:</strong></p>
<p>%s</p>
<p><a href="%s">View on Reddit</a></p>
</body>
</html>
`, h.from, to, subject, matchType, keyword, subreddit, author, title, emailBody,
		url)

	auth := smtp.PlainAuth("", h.from, h.password, h.smtpHost)
	addr := fmt.Sprintf("%s:%s", h.smtpHost, h.smtpPort)
	err := smtp.SendMail(addr, auth, h.from, []string{to}, []byte(message))
	if err != nil {
		h.logger.Error("Failed to send email", "error", err)
		return err
	}

	h.logger.Info("Email sent", "keyword", keyword, "subreddit", subreddit, "type", matchType)
	return nil
}
