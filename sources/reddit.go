package sources

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/config"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/data/repos"
	"github.com/kova98/feedgrep.api/models"
)

type RedditPoller struct {
	logger          *slog.Logger
	httpClient      *http.Client
	keywordRepo     *repos.KeywordRepo
	matchRepo       *repos.MatchRepo
	seenPosts       map[string]bool
	seenComments    map[string]bool
	subscriptions   []keywordSubscription
	pollInterval    time.Duration
	keywordInterval time.Duration
}
type keywordSubscription struct {
	id      int
	userID  uuid.UUID
	keyword string
}

func NewRedditPoller(logger *slog.Logger, httpClient *http.Client, keywordRepo *repos.KeywordRepo, matchRepo *repos.MatchRepo) *RedditPoller {
	return &RedditPoller{
		logger:          logger,
		httpClient:      httpClient,
		keywordRepo:     keywordRepo,
		matchRepo:       matchRepo,
		seenPosts:       make(map[string]bool),
		seenComments:    make(map[string]bool),
		pollInterval:    time.Duration(config.Config.PollIntervalSeconds) * time.Second,
		keywordInterval: time.Minute,
	}
}

func (h *RedditPoller) StartPolling(ctx context.Context) {
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

func (h *RedditPoller) pollOnce() {
	h.pollPosts()
	time.Sleep(2 * time.Second) // Delay between requests to avoid rate limiting
	h.pollComments()
}

func (h *RedditPoller) pollPosts() {
	matches := make([]data.Match, 0, 32)
	listing, err := h.fetchReddit("https://www.reddit.com/r/all/new/.json?limit=100")
	if err != nil {
		h.logger.Error("poll posts:", "error", err)
		return
	}

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
				input, buildErr := h.buildMatch(post, sub, false)
				if buildErr != nil {
					h.logger.Error("failed to build match", "error", buildErr, "post_id", post.ID)
					continue
				}
				matches = append(matches, input)
			}
		}
	}

	if len(matches) > 0 {
		if err := h.matchRepo.CreateMatches(matches); err != nil {
			h.logger.Error("failed to store matches", "error", err)
		}
	}

	h.logger.Info("processed posts", "new_matches", len(matches), "total_seen", len(h.seenPosts))
}

func (h *RedditPoller) pollComments() {
	matches := make([]data.Match, 0, 32)
	listing, err := h.fetchReddit("https://www.reddit.com/r/all/comments/.json?limit=100")
	if err != nil {
		h.logger.Error("poll comments:", "error", err)
		return
	}

	for _, child := range listing.Data.Children {
		comment := child.Data

		if h.seenComments[comment.ID] {
			continue
		}
		h.seenComments[comment.ID] = true

		body := strings.ToLower(comment.Body)

		for _, sub := range h.subscriptions {
			if strings.Contains(body, sub.keyword) {
				input, buildErr := h.buildMatch(comment, sub, true)
				if buildErr != nil {
					h.logger.Error("failed to build match", "error", buildErr, "comment_id", comment.ID)
					continue
				}
				matches = append(matches, input)
			}
		}
	}

	if len(matches) > 0 {
		if err := h.matchRepo.CreateMatches(matches); err != nil {
			h.logger.Error("failed to store matches", "error", err)
		}
	}

	h.logger.Info("processed comments", "new_matches", len(matches), "total_seen", len(h.seenComments))
}

func (h *RedditPoller) fetchReddit(url string) (*models.RedditListing, error) {
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

func (h *RedditPoller) loadKeywords() {
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
		active = append(active, keywordSubscription{
			id:      keyword.ID,
			userID:  keyword.UserID,
			keyword: kw,
		})
	}

	h.subscriptions = active
	h.logger.Info("refreshed subscriptions", "count", len(h.subscriptions))
}

func (h *RedditPoller) buildMatch(item models.RedditPost, sub keywordSubscription, isComment bool) (data.Match, error) {
	url := fmt.Sprintf("https://reddit.com%s", item.Permalink)
	matchHash := buildMatchHash(sub.userID, sub.id, "reddit", url)

	payload := repos.MatchPayload{
		Keyword:   sub.keyword,
		Subreddit: item.Subreddit,
		Author:    item.Author,
		Title:     item.Title,
		Body:      item.Selftext,
		IsComment: isComment,
	}
	if isComment {
		payload.Title = ""
		payload.Body = item.Body
	}

	match, err := repos.NewMatch(
		sub.userID,
		sql.NullInt64{Int64: int64(sub.id), Valid: true},
		"reddit",
		sql.NullString{String: url, Valid: url != ""},
		matchHash,
		payload,
	)
	if err != nil {
		return data.Match{}, err
	}

	return match, nil
}

func buildMatchHash(userID uuid.UUID, keywordID int, source, url string) string {
	input := fmt.Sprintf("%s:%d:%s:%s", userID.String(), keywordID, source, url)
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
