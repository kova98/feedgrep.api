package sources

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/config"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/data/repos"
	"github.com/kova98/feedgrep.api/enums"
	"github.com/kova98/feedgrep.api/matchers"
	"github.com/kova98/feedgrep.api/models"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:122.0) Gecko/20100101 Firefox/122.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Edg/121.0.0.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 11.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Vivaldi/6.5.3206.39",
}

type RedditPoller struct {
	logger              *slog.Logger
	proxyPool           *ProxyPool
	keywordRepo         *repos.KeywordRepo
	matchRepo           *repos.MatchRepo
	seenPosts           map[string]bool
	seenPostsMu         sync.Mutex
	seenComments        map[string]bool
	seenCommentsMu      sync.Mutex
	subscriptions       []keywordSubscription
	postPollInterval    time.Duration
	commentPollInterval time.Duration
	keywordInterval     time.Duration

	// Pagination: track last newest ID to use with "before" param (posts only, comments don't support it)
	lastNewestPostID   string
	lastNewestPostIDMu sync.Mutex

	// Throughput stats (reset every minute)
	statsPostsNew     atomic.Int64
	statsCommentsNew  atomic.Int64
	statsPostPolls    atomic.Int64
	statsCommentPolls atomic.Int64
	statsLastReset    time.Time
	statsLastResetMu  sync.Mutex
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

func NewRedditPoller(logger *slog.Logger, proxyPool *ProxyPool, keywordRepo *repos.KeywordRepo, matchRepo *repos.MatchRepo) *RedditPoller {
	return &RedditPoller{
		logger:              logger,
		proxyPool:           proxyPool,
		keywordRepo:         keywordRepo,
		matchRepo:           matchRepo,
		seenPosts:           make(map[string]bool),
		seenComments:        make(map[string]bool),
		postPollInterval:    time.Duration(config.Config.PostPollIntervalMs) * time.Millisecond,
		commentPollInterval: time.Duration(config.Config.CommentPollIntervalMs) * time.Millisecond,
		keywordInterval:     time.Minute,
		statsLastReset:      time.Now(),
	}
}

func (h *RedditPoller) StartPolling(ctx context.Context) {
	h.loadKeywords()
	h.logger.Info("starting reddit polling",
		"subscriptions", len(h.subscriptions),
		"post_interval", h.postPollInterval.Seconds(),
		"comment_interval", h.commentPollInterval.Seconds())

	postTicker := time.NewTicker(h.postPollInterval)
	commentTicker := time.NewTicker(h.commentPollInterval)
	keywordTicker := time.NewTicker(h.keywordInterval)
	statsTicker := time.NewTicker(time.Minute)
	defer postTicker.Stop()
	defer commentTicker.Stop()
	defer keywordTicker.Stop()
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("stopping reddit polling")
			return
		case <-postTicker.C:
			go func() {
				if !h.pollPosts() {
					time.Sleep(100 * time.Millisecond)
					h.pollPosts()
				}
			}()
		case <-keywordTicker.C:
			h.loadKeywords()
		case <-statsTicker.C:
			h.logProxyStats()
		case <-commentTicker.C:
			go func() {
				if !h.pollComments() {
					time.Sleep(100 * time.Millisecond)
					h.pollComments()
				}
			}()
		}
	}
}

func (h *RedditPoller) logProxyStats() {
	// Log throughput stats
	h.statsLastResetMu.Lock()
	elapsed := time.Since(h.statsLastReset)
	postsNew := h.statsPostsNew.Swap(0)
	commentsNew := h.statsCommentsNew.Swap(0)
	postPolls := h.statsPostPolls.Swap(0)
	commentPolls := h.statsCommentPolls.Swap(0)
	h.statsLastReset = time.Now()
	h.statsLastResetMu.Unlock()

	elapsedSec := elapsed.Seconds()
	h.logger.Info("throughput stats",
		"elapsed_sec", fmt.Sprintf("%.1f", elapsedSec),
		"posts_new", postsNew,
		"posts_per_min", fmt.Sprintf("%.1f", float64(postsNew)/elapsedSec*60),
		"post_polls", postPolls,
		"comments_new", commentsNew,
		"comments_per_min", fmt.Sprintf("%.1f", float64(commentsNew)/elapsedSec*60),
		"comment_polls", commentPolls,
		"seen_posts_total", len(h.seenPosts),
		"seen_comments_total", len(h.seenComments),
	)

	stats := h.proxyPool.GetStats()

	// Sort by success rate
	type proxyStat struct {
		host      string
		successes int
		failures  int
		rate      float64
	}

	var sorted []proxyStat
	for host, s := range stats {
		total := s.Successes + s.Failures
		rate := 0.0
		if total > 0 {
			rate = float64(s.Successes) / float64(total) * 100
		}
		sorted = append(sorted, proxyStat{host, s.Successes, s.Failures, rate})
	}

	if len(sorted) == 0 {
		return
	}

	// Sort by success rate descending
	slices.SortFunc(sorted, func(a, b proxyStat) int {
		return cmp.Compare(b.rate, a.rate)
	})

	h.logger.Info("proxy stats - top performers")
	for i := 0; i < min(5, len(sorted)); i++ {
		s := sorted[i]
		h.logger.Info("proxy", "host", s.host, "successes", s.successes, "failures", s.failures, "rate", fmt.Sprintf("%.1f%%", s.rate))
	}

	if len(sorted) > 5 {
		h.logger.Info("proxy stats - worst performers")
		for i := max(0, len(sorted)-5); i < len(sorted); i++ {
			s := sorted[i]
			h.logger.Info("proxy", "host", s.host, "successes", s.successes, "failures", s.failures, "rate", fmt.Sprintf("%.1f%%", s.rate))
		}
	}
}

func (h *RedditPoller) pollPosts() bool {
	matches := make([]data.Match, 0, 32)

	url := "https://www.reddit.com/r/all/new/.json?limit=100"
	h.lastNewestPostIDMu.Lock()
	if h.lastNewestPostID != "" {
		url += "&before=" + h.lastNewestPostID
	}
	h.lastNewestPostIDMu.Unlock()

	client, proxyHost := h.proxyPool.Next()
	listing, requestMs, err := h.fetchReddit(url, client, proxyHost)
	processingStart := time.Now()
	if err != nil {
		h.logger.Debug("poll posts", "proxy", proxyHost, "request_ms", requestMs, "error", truncateError(err))
		return false
	}

	if len(listing.Data.Children) == 0 {
		return true
	}

	newestItem := listing.Data.Children[0].Data
	newCount := 0

	h.seenPostsMu.Lock()
	for _, child := range listing.Data.Children {
		post := child.Data
		if h.seenPosts[post.ID] {
			continue
		}
		h.seenPosts[post.ID] = true
		newCount++

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
	h.seenPostsMu.Unlock()

	if len(matches) > 0 {
		if err := h.matchRepo.CreateMatches(matches); err != nil {
			h.logger.Error("failed to store matches", "error", err)
		}
	}

	// Update last newest ID for next poll
	h.lastNewestPostIDMu.Lock()
	h.lastNewestPostID = newestItem.Name
	h.lastNewestPostIDMu.Unlock()

	// Update stats
	h.statsPostsNew.Add(int64(newCount))
	h.statsPostPolls.Add(1)

	// Measure lag between now and newest post
	lagSeconds := time.Now().Unix() - int64(newestItem.CreatedUTC)
	processingMs := time.Since(processingStart).Milliseconds()
	h.logger.Debug("processed posts", "proxy", proxyHost, "new", newCount, "matches", len(matches), "lag_seconds", lagSeconds, "request_ms", requestMs, "processing_ms", processingMs)
	return true
}

func (h *RedditPoller) pollComments() bool {
	matches := make([]data.Match, 0, 32)

	// /r/all/comments doesn't support pagination
	client, proxyHost := h.proxyPool.Next()
	listing, requestMs, err := h.fetchReddit("https://www.reddit.com/r/all/comments/.json?limit=100", client, proxyHost)
	processingStart := time.Now()
	if err != nil {
		h.logger.Debug("poll comments", "proxy", proxyHost, "request_ms", requestMs, "error", truncateError(err))
		return false
	}

	if len(listing.Data.Children) == 0 {
		return true
	}

	newestItem := listing.Data.Children[0].Data
	newCount := 0

	h.seenCommentsMu.Lock()
	for _, child := range listing.Data.Children {
		comment := child.Data

		if h.seenComments[comment.ID] {
			continue
		}
		h.seenComments[comment.ID] = true
		newCount++

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
	h.seenCommentsMu.Unlock()

	if len(matches) > 0 {
		if err := h.matchRepo.CreateMatches(matches); err != nil {
			h.logger.Error("failed to store matches", "error", err)
		}
	}

	// Update stats
	h.statsCommentsNew.Add(int64(newCount))
	h.statsCommentPolls.Add(1)

	// Measure lag between now and newest comment
	lagSeconds := time.Now().Unix() - int64(newestItem.CreatedUTC)
	processingMs := time.Since(processingStart).Milliseconds()

	if newCount == len(listing.Data.Children) && newCount >= 100 {
		h.logger.Debug("all comments are new, likely missing some", "proxy", proxyHost, "new", newCount)
	}

	h.logger.Debug("processed comments", "proxy", proxyHost, "new", newCount, "matches", len(matches), "lag_seconds", lagSeconds, "request_ms", requestMs, "processing_ms", processingMs)
	return true
}

func (h *RedditPoller) fetchReddit(url string, client *http.Client, proxyHost string) (*models.RedditListing, int64, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}

	// Make the request look like a real browser to avoid blocks
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	start := time.Now()
	resp, err := client.Do(req)
	requestMs := time.Since(start).Milliseconds()
	if err != nil {
		return nil, requestMs, fmt.Errorf("(%dms) %w", requestMs, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.proxyPool.MarkFailure(proxyHost)
		if resp.StatusCode == http.StatusTooManyRequests {
			h.proxyPool.MarkRateLimited(proxyHost)
		}
		return nil, requestMs, fmt.Errorf("status %d", resp.StatusCode)
	}

	var listing models.RedditListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		h.proxyPool.MarkFailure(proxyHost)
		return nil, requestMs, err
	}

	h.proxyPool.MarkSuccess(proxyHost)
	return &listing, requestMs, nil
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

func truncateError(err error) error {
	msg := err.Error()
	if len(msg) > 300 {
		return fmt.Errorf("%s...", msg[:300])
	}
	return err
}
