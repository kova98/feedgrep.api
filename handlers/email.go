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

	message := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8

<html>
<body style="margin:0; background:#ffffff; font-family:Arial, Helvetica, sans-serif; color:#202124;">
  <div style="max-width:820px; margin:0 auto; padding:48px 24px; box-sizing:border-box;">
    <div style="background:#ffffff; border:1px solid #e0e0e0; border-radius:12px; box-shadow:0 2px 10px rgba(0,0,0,0.06); overflow:hidden;">
      <div style="padding:20px 24px 8px; border-bottom:1px solid #eceff1;">
        <div style="font-size:18px; font-weight:600; margin:0 0 8px 0;">%s</div>
        <div style="display:flex; align-items:center; gap:12px; font-size:13px; color:#5f6368;">
          <span style="background:#f1f3f4; border-radius:999px; padding:2px 10px; font-size:12px; color:#3c4043;">Inbox</span>
          <span>From: %s</span>
          <span>·</span>
          <span>to you</span>
          <span>·</span>
          <span>just now</span>
        </div>
      </div>
      <div style="padding:20px 24px 26px; font-size:14px; line-height:1.5;">
        <div style="color:#5f6368; font-size:13px; margin-bottom:10px;"><strong>r/%s</strong> · u/%s · %s</div>
        <p><strong>Keyword:</strong> %s</p>
        %s
        <p>%s</p>
        <p><a href="%s" style="color:#1a73e8; text-decoration:none;">View on Reddit</a></p>
      </div>
    </div>
  </div>
</body>
</html>
`, h.from, to, subject, subject, h.from, subreddit, author, matchType, keyword, title, emailBody,
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
