package notifiers

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"text/template"

	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/models"
)

//go:embed templates/reddit_match.html templates/reddit_digest.html
var emailTemplates embed.FS

var redditTemplates = template.Must(template.New("emails").ParseFS(emailTemplates, "templates/*.html"))

type Mailer struct {
	smtpHost string
	smtpPort string
	from     string
	password string
	appBase  string
}

func NewMailer(smtpHost, smtpPort, from, password, appBase string) *Mailer {
	return &Mailer{
		smtpHost: smtpHost,
		smtpPort: smtpPort,
		from:     from,
		password: password,
		appBase:  strings.TrimRight(appBase, "/"),
	}
}

func (h *Mailer) RedditMatchEmail(email string, match data.Match) (models.Email, error) {
	var payload data.RedditData
	if err := json.Unmarshal(match.DataRaw, &payload); err != nil {
		return models.Email{}, err
	}

	subject := "feedgrep: new mentions"

	matchType := "Post"
	if payload.IsComment {
		matchType = "Comment"
	}

	body := strings.TrimSpace(payload.Body)
	if len(body) > 500 {
		body = body[:500] + "..."
	}
	body = strings.ReplaceAll(body, "\n", "<br>")

	url := "https://reddit.com" + payload.Permalink

	var buf bytes.Buffer
	tmplData := struct {
		Keyword          string
		Subreddit        string
		Author           string
		MatchType        string
		Title            string
		Body             string
		URL              string
		KeywordConfigURL string
	}{
		Keyword:          payload.Keyword,
		Subreddit:        payload.Subreddit,
		Author:           payload.Author,
		MatchType:        matchType,
		Title:            payload.Title,
		Body:             body,
		URL:              url,
		KeywordConfigURL: h.keywordConfigURL(match.KeywordID),
	}
	if err := redditTemplates.ExecuteTemplate(&buf, "reddit_match.html", tmplData); err != nil {
		return models.Email{}, fmt.Errorf("render reddit match template: %w", err)
	}

	return models.Email{
		To:      email,
		Subject: subject,
		Body:    buf.String(),
	}, nil
}

func (h *Mailer) RedditDigestEmail(email string, matches []data.Match) (models.Email, error) {
	type digestItem struct {
		Keyword          string
		Subreddit        string
		Author           string
		Title            string
		Body             string
		URL              string
		MatchType        string
		KeywordConfigURL string
	}

	items := make([]digestItem, 0, 10)
	keywordSet := make(map[string]struct{})
	total := 0
	for _, match := range matches {
		var payload data.RedditData
		if err := json.Unmarshal(match.DataRaw, &payload); err != nil {
			continue
		}
		total++
		keywordSet[payload.Keyword] = struct{}{}
		if len(items) >= 10 {
			continue
		}

		matchType := "Post"
		if payload.IsComment {
			matchType = "Comment"
		}

		url := payload.Permalink
		if !strings.HasPrefix(url, "http") {
			url = "https://reddit.com" + url
		}

		title := strings.TrimSpace(payload.Title)

		body := strings.TrimSpace(payload.Body)
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		body = strings.ReplaceAll(body, "\n", "<br>")

		items = append(items, digestItem{
			Keyword:          payload.Keyword,
			Subreddit:        payload.Subreddit,
			Author:           payload.Author,
			Title:            title,
			Body:             body,
			URL:              url,
			MatchType:        matchType,
			KeywordConfigURL: h.keywordConfigURL(match.KeywordID),
		})
	}

	if total == 0 {
		return models.Email{}, fmt.Errorf("no valid matches")
	}

	keywords := make([]string, 0, len(keywordSet))
	for k := range keywordSet {
		keywords = append(keywords, k)
	}

	remaining := total - len(items)
	if remaining < 0 {
		remaining = 0
	}

	var buf bytes.Buffer
	tmplData := struct {
		Items     []digestItem
		Keywords  []string
		Total     int
		Remaining int
	}{
		Items:     items,
		Keywords:  keywords,
		Total:     total,
		Remaining: remaining,
	}
	if err := redditTemplates.ExecuteTemplate(&buf, "reddit_digest.html", tmplData); err != nil {
		return models.Email{}, fmt.Errorf("render reddit digest template: %w", err)
	}

	subject := "feedgrep: new mentions"

	return models.Email{
		To:      email,
		Subject: subject,
		Body:    buf.String(),
	}, nil
}

func (h *Mailer) Send(mail models.Email) error {
	message := fmt.Sprintf(`From: feedgrep <%s>
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8

%s`, h.from, mail.To, mail.Subject, mail.Body)

	auth := smtp.PlainAuth("", h.from, h.password, h.smtpHost)
	addr := fmt.Sprintf("%s:%s", h.smtpHost, h.smtpPort)
	err := smtp.SendMail(addr, auth, h.from, []string{mail.To}, []byte(message))
	if err != nil {
		slog.Error("Failed to send email", "error", err)
		return err
	}

	slog.Info("email sent", "recipient", mail.To, "subject", mail.Subject)
	return nil
}

func (h *Mailer) keywordConfigURL(keywordID int) string {
	if h.appBase == "" || keywordID <= 0 {
		return ""
	}

	return fmt.Sprintf("%s/keywords/%d/edit", h.appBase, keywordID)
}
