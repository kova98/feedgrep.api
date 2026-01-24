package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kova98/feedgrep.api/config"
	"github.com/kova98/feedgrep.api/data/repos"
	"github.com/kova98/feedgrep.api/models"
)

type RedditHandler struct {
	logger          *slog.Logger
	httpClient      *http.Client
	emailHandler    *EmailHandler
	keywordRepo     *repos.KeywordRepo
	seenPosts       map[string]bool
	seenComments    map[string]bool
	subscriptions   []keywordSubscription
	pollInterval    time.Duration
	keywordInterval time.Duration
}
type keywordSubscription struct {
	keyword string
	email   string
}

func NewRedditHandler(logger *slog.Logger, emailHandler *EmailHandler, httpClient *http.Client, keywordRepo *repos.KeywordRepo) *RedditHandler {
	return &RedditHandler{
		logger:          logger,
		httpClient:      httpClient,
		emailHandler:    emailHandler,
		keywordRepo:     keywordRepo,
		seenPosts:       make(map[string]bool),
		seenComments:    make(map[string]bool),
		pollInterval:    time.Duration(config.Config.PollIntervalSeconds) * time.Second,
		keywordInterval: time.Minute,
	}
}

func (h *RedditHandler) StartPolling(ctx context.Context) {
	h.loadKeywords()
	h.logger.Info("starting reddit polling", "subscriptions", len(h.subscriptions), "interval", h.pollInterval.Seconds())

	pollTicker := time.NewTicker(h.pollInterval)
	keywordTicker := time.NewTicker(h.keywordInterval)
	defer pollTicker.Stop()
	defer keywordTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			h.logger.Info("stopping reddit polling")
			return
		case <-pollTicker.C:
			h.pollOnce()
		case <-keywordTicker.C:
			h.loadKeywords()
		}
	}
}

func (h *RedditHandler) pollOnce() {
	h.pollPosts()
	time.Sleep(2 * time.Second) // Delay between requests to avoid rate limiting
	h.pollComments()
}

func (h *RedditHandler) pollPosts() {
	listing, err := h.fetchReddit("https://www.reddit.com/r/all/new/.json?limit=100")
	if err != nil {
		h.logger.Error("poll posts:", "error", err)
		return
	}

	newMatches := 0
	for _, child := range listing.Data.Children {
		post := child.Data
		if h.seenPosts[post.ID] {
			continue
		}
		h.seenPosts[post.ID] = true

		title := strings.ToLower(post.Title)
		body := strings.ToLower(post.Selftext)

		for _, sub := range h.subscriptions {
			if strings.Contains(title, sub.keyword) || strings.Contains(body, sub.keyword) {
				h.onMatched(post, sub.keyword, sub.email)
				newMatches++
			}
		}
	}

	h.logger.Info("processed posts", "new_matches", newMatches, "total_seen", len(h.seenPosts))
}

func (h *RedditHandler) pollComments() {
	listing, err := h.fetchReddit("https://www.reddit.com/r/all/comments/.json?limit=100")
	if err != nil {
		h.logger.Error("poll comments:", "error", err)
		return
	}

	newMatches := 0
	for _, child := range listing.Data.Children {
		comment := child.Data

		if h.seenComments[comment.ID] {
			continue
		}
		h.seenComments[comment.ID] = true

		body := strings.ToLower(comment.Body)

		for _, sub := range h.subscriptions {
			if strings.Contains(body, sub.keyword) {
				h.printCommentMatch(comment, sub.keyword, sub.email)
				newMatches++
			}
		}
	}

	h.logger.Info("processed comments", "new_matches", newMatches, "total_seen", len(h.seenComments))
}

func (h *RedditHandler) fetchReddit(url string) (*models.RedditListing, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Make the request look like a real browser to avoid blocks
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("reddit returned status %d: %s", resp.StatusCode, string(body))
	}

	var listing models.RedditListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, err
	}

	return &listing, nil
}

func (h *RedditHandler) loadKeywords() {
	keywords, err := h.keywordRepo.GetActiveKeywordsWithEmails()
	if err != nil {
		h.logger.Error("failed to refresh subscriptions", "error", err)
		return
	}

	active := make([]keywordSubscription, 0, len(keywords))
	for _, keyword := range keywords {
		kw := strings.TrimSpace(strings.ToLower(keyword.Keyword))
		email := strings.TrimSpace(keyword.Email)
		if kw == "" || email == "" {
			continue
		}
		active = append(active, keywordSubscription{keyword: kw, email: email})
	}

	h.subscriptions = active
	h.logger.Info("refreshed subscriptions", "count", len(h.subscriptions))
}

func (h *RedditHandler) onMatched(post models.RedditPost, keyword, email string) {
	url := fmt.Sprintf("https://reddit.com%s", post.Permalink)
	err := h.emailHandler.SendEmail(
		email,
		keyword,
		post.Subreddit,
		post.Author,
		post.Title,
		post.Selftext,
		url,
		false, // isComment
	)
	if err != nil {
		h.logger.Error("Failed to send email for post match", "error", err, "post_id", post.ID)
	}
}

func (h *RedditHandler) printCommentMatch(comment models.RedditPost, keyword, email string) {
	url := fmt.Sprintf("https://reddit.com%s", comment.Permalink)
	err := h.emailHandler.SendEmail(
		email,
		keyword,
		comment.Subreddit,
		comment.Author,
		"", // no title for comments
		comment.Body,
		url,
		true, // isComment
	)
	if err != nil {
		h.logger.Error("Failed to send email for comment match", "error", err, "comment_id", comment.ID)
	}
}
