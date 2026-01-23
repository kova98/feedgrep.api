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

	"github.com/kova98/feedgrep.api/models"
)

type RedditHandler struct {
	logger       *slog.Logger
	httpClient   *http.Client
	emailHandler *EmailHandler
	seenPosts    map[string]bool
	seenComments map[string]bool
	keywords     []string
}

func NewRedditHandler(logger *slog.Logger, emailHandler *EmailHandler) *RedditHandler {
	return &RedditHandler{
		logger:       logger,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		emailHandler: emailHandler,
		seenPosts:    make(map[string]bool),
		seenComments: make(map[string]bool),
		keywords:     []string{"hello", "test"},
	}
}

func (h *RedditHandler) StartPolling(ctx context.Context) {
	interval := 10 * time.Second

	h.logger.Info("Starting Reddit polling", "keywords", h.keywords, "interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			h.logger.Info("Stopping Reddit polling...")
			return
		case <-ticker.C:
			h.pollOnce()
		}
	}
}

func (h *RedditHandler) pollOnce() {
	h.pollPosts()
	h.pollComments()
}

func (h *RedditHandler) pollPosts() {
	listing, err := h.fetchReddit("https://www.reddit.com/r/all/new/.json?limit=100")
	if err != nil {
		h.logger.Error("Failed to fetch posts", "error", err)
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

		for _, keyword := range h.keywords {
			if strings.Contains(title, keyword) || strings.Contains(body, keyword) {
				h.onMatched(post, keyword)
				newMatches++
			}
		}
	}

	h.logger.Info("processed posts", "new_matches", newMatches, "total_seen", len(h.seenPosts))
}

func (h *RedditHandler) pollComments() {
	listing, err := h.fetchReddit("https://www.reddit.com/r/all/comments/.json?limit=100")
	if err != nil {
		h.logger.Error("Failed to fetch comments", "error", err)
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

		for _, keyword := range h.keywords {
			if strings.Contains(body, keyword) {
				h.printCommentMatch(comment, keyword)
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

	// Reddit requires a user agent
	req.Header.Set("User-Agent", "feedgrep")

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

func (h *RedditHandler) onMatched(post models.RedditPost, keyword string) {
	url := fmt.Sprintf("https://reddit.com%s", post.Permalink)
	err := h.emailHandler.SendEmail(
		"roko@barelytics.io",
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

func (h *RedditHandler) printCommentMatch(comment models.RedditPost, keyword string) {
	url := fmt.Sprintf("https://reddit.com%s", comment.Permalink)
	err := h.emailHandler.SendEmail(
		"roko@barelytics.io",
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
