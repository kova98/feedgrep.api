package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"github.com/kova98/feedgrep.api/enums"
	"github.com/kova98/feedgrep.api/matchers"
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
	id        int
	userID    uuid.UUID
	keyword   string
	matchMode enums.MatchMode
	filters   *data.RedditFilters
}

func (s *keywordSubscription) Matches(text, subreddit string) (bool, error) {
	textLower := strings.ToLower(text)

	if s.matchMode == enums.MatchModeInvalid {
		return false, errors.New(string("invalid match mode: " + s.matchMode))
	}

	if s.matchMode == enums.MatchModeExact && !matchers.MatchesWholeWord(textLower, s.keyword) {
		return false, nil
	}

	if s.matchMode == enums.MatchModeBroad && !matchers.MatchesPartially(textLower, s.keyword) {
		return false, nil
	}

	if s.filters != nil {
		match, err := matchers.MatchesSubreddit(*s.filters, subreddit)
		if err != nil {
			return false, err
		}
		if !match {
			return false, nil
		}
	}

	return true, nil
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
		if len(err.Error()) > 300 {
			err = fmt.Errorf("%s...", err.Error()[:300])
		}
		h.logger.Error("poll posts:", "error", err)
		return
	}

	for _, child := range listing.Data.Children {
		post := child.Data
		if h.seenPosts[post.ID] {
			continue
		}
		h.seenPosts[post.ID] = true

		text := post.Title + " " + post.Selftext

		for _, sub := range h.subscriptions {
			subMatches, err := sub.Matches(text, post.Subreddit)
			if err != nil {
				h.logger.Error("failed to check match", "error", err, "post_id", post.ID)
				continue
			}
			if !subMatches {
				continue
			}

			match, err := h.makeMatch(post, sub, false)
			if err != nil {
				h.logger.Error("failed to make match", "error", err, "post_id", post.ID)
				continue
			}
			matches = append(matches, match)
		}
	}

	if len(matches) > 0 {
		if err := h.matchRepo.CreateMatches(matches); err != nil {
			h.logger.Error("failed to store matches", "error", err)
		}
	}

	h.logger.Debug("processed posts", "new_matches", len(matches), "total_seen", len(h.seenPosts))
}

func (h *RedditPoller) pollComments() {
	matches := make([]data.Match, 0, 32)
	listing, err := h.fetchReddit("https://www.reddit.com/r/all/comments/.json?limit=100")
	if err != nil {
		if len(err.Error()) > 300 {
			err = fmt.Errorf("%s...", err.Error()[:300])
		}
		h.logger.Error("poll comments:", "error", err)
		return
	}

	for _, child := range listing.Data.Children {
		comment := child.Data

		if h.seenComments[comment.ID] {
			continue
		}
		h.seenComments[comment.ID] = true

		for _, sub := range h.subscriptions {
			subMatches, err := sub.Matches(comment.Body, comment.Subreddit)
			if err != nil {
				h.logger.Error("failed to check match", "error", err, "comment_id", comment.ID)
				continue
			}
			if !subMatches {
				continue
			}
			match, err := h.makeMatch(comment, sub, true)
			if err != nil {
				h.logger.Error("failed to make match", "error", err, "comment_id", comment.ID)
				continue
			}
			matches = append(matches, match)
		}
	}

	if len(matches) > 0 {
		if err := h.matchRepo.CreateMatches(matches); err != nil {
			h.logger.Error("failed to store matches", "error", err)
		}
	}

	h.logger.Debug("processed comments", "new_matches", len(matches), "total_seen", len(h.seenComments))
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
			id:        keyword.ID,
			userID:    keyword.UserID,
			keyword:   kw,
			matchMode: keyword.MatchMode,
			filters:   keyword.Filters.Reddit,
		})
	}

	h.subscriptions = active
	h.logger.Info("refreshed subscriptions", "count", len(h.subscriptions))
}

func (h *RedditPoller) makeMatch(item models.RedditPost, sub keywordSubscription, isComment bool) (data.Match, error) {
	redditData := data.RedditData{
		Keyword:   sub.keyword,
		Subreddit: item.Subreddit,
		Author:    item.Author,
		Title:     item.Title,
		Body:      item.Selftext,
		IsComment: isComment,
		Permalink: item.Permalink,
	}
	if isComment {
		redditData.Title = ""
		redditData.Body = item.Body
	}
	matchHash := buildMatchHash(sub.userID, sub.id, item.Permalink)
	match, err := data.NewMatch(
		sub.userID,
		sub.id,
		enums.SourceReddit,
		matchHash,
		redditData,
	)
	if err != nil {
		return data.Match{}, err
	}

	return match, nil
}

func buildMatchHash(userID uuid.UUID, keywordID int, url string) string {
	input := fmt.Sprintf("%s:%d:%s:%s", userID.String(), keywordID, enums.SourceReddit, url)
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
