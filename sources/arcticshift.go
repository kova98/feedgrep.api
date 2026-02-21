package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	neturl "net/url"
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

const (
	arcticShiftBaseURL        = "https://arctic-shift.photon-reddit.com/api"
	arcticShiftPostsFields    = "id,subreddit,author,title,selftext,created_utc"
	arcticShiftCommentsFields = "id,subreddit,author,body,link_id,parent_id,created_utc"
)

type ArcticShiftPoller struct {
	logger      *slog.Logger
	keywordRepo *repos.KeywordRepo
	matchRepo   *repos.MatchRepo
	client      *http.Client

	subscriptions       []keywordSubscription
	postPollInterval    time.Duration
	commentPollInterval time.Duration
	lastPostCreated     int64
	lastCommentCreated  int64
	postsTotal          int64
	commentsTotal       int64
	postsWindow         int64
	commentsWindow      int64
	windowStart         time.Time
}

func NewArcticShiftPoller(logger *slog.Logger, keywordRepo *repos.KeywordRepo, matchRepo *repos.MatchRepo) *ArcticShiftPoller {
	interval := time.Duration(config.Config.PostPollIntervalMs) * time.Millisecond

	return &ArcticShiftPoller{
		logger:              logger,
		keywordRepo:         keywordRepo,
		matchRepo:           matchRepo,
		client:              &http.Client{Timeout: 15 * time.Second},
		postPollInterval:    interval,
		commentPollInterval: interval,
		windowStart:         time.Now(),
	}
}

func (h *ArcticShiftPoller) StartPolling(ctx context.Context) {
	h.logger.Info("starting arcticshift polling",
		"post_interval", h.postPollInterval.Seconds(),
		"comment_interval", h.commentPollInterval.Seconds())

	h.loadKeywords()

	ticker := time.NewTicker(h.postPollInterval)
	keywordTicker := time.NewTicker(1 * time.Minute)
	statsTicker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	defer keywordTicker.Stop()
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("stopping arcticshift polling")
			return
		case <-ticker.C:
			if !h.pollPosts() {
				time.Sleep(100 * time.Millisecond)
				h.pollPosts()
			}
			if !h.pollComments() {
				time.Sleep(100 * time.Millisecond)
				h.pollComments()
			}
		case <-keywordTicker.C:
			h.loadKeywords()
		case <-statsTicker.C:
			h.logThroughput()
		}
	}
}

func (h *ArcticShiftPoller) pollPosts() bool {
	matches := make([]data.Match, 0, 32)

	url := fmt.Sprintf("%s/posts/search?limit=auto&sort=desc&fields=%s", arcticShiftBaseURL, arcticShiftPostsFields)
	if h.lastPostCreated > 0 {
		after := time.Unix(h.lastPostCreated, 0).UTC().Format(time.RFC3339)
		url = fmt.Sprintf("%s/posts/search?limit=auto&sort=asc&after=%s&fields=%s", arcticShiftBaseURL, neturl.QueryEscape(after), arcticShiftPostsFields)
	}

	var resp models.ArcticShiftSearchResponse[models.ArcticShiftPost]
	requestMs, err := h.fetchArcticShift(url, &resp)
	processingStart := time.Now()
	if err != nil {
		h.logger.Info("poll posts", "request_ms", requestMs, "error", truncateError(err))
		return false
	}

	if len(resp.Data) == 0 {
		return true
	}

	var newestPostUTC int64
	processedPosts := int64(0)
	for _, post := range resp.Data {
		if post.ID == "" {
			continue
		}
		processedPosts++
		if post.CreatedUTC > newestPostUTC {
			newestPostUTC = post.CreatedUTC
		}

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

			match, err := h.makePostMatch(post, sub)
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
	if newestPostUTC > h.lastPostCreated {
		h.lastPostCreated = newestPostUTC
	}
	h.postsTotal += processedPosts
	h.postsWindow += processedPosts

	processingMs := time.Since(processingStart).Milliseconds()
	lagSeconds := int64(0)
	if newestPostUTC > 0 {
		lagSeconds = time.Now().Unix() - newestPostUTC
	}
	h.logger.Info("processed posts", "count", processedPosts, "matches", len(matches), "request_ms", requestMs, "processing_ms", processingMs, "lag_seconds", lagSeconds)
	return true
}

func (h *ArcticShiftPoller) pollComments() bool {
	matches := make([]data.Match, 0, 32)

	url := fmt.Sprintf("%s/comments/search?limit=auto&sort=desc&fields=%s", arcticShiftBaseURL, arcticShiftCommentsFields)
	if h.lastCommentCreated > 0 {
		after := time.Unix(h.lastCommentCreated, 0).UTC().Format(time.RFC3339)
		url = fmt.Sprintf("%s/comments/search?limit=auto&sort=asc&after=%s&fields=%s", arcticShiftBaseURL, neturl.QueryEscape(after), arcticShiftCommentsFields)
	}

	var resp models.ArcticShiftSearchResponse[models.ArcticShiftComment]
	requestMs, err := h.fetchArcticShift(url, &resp)
	processingStart := time.Now()
	if err != nil {
		h.logger.Debug("poll comments", "request_ms", requestMs, "error", truncateError(err))
		return false
	}

	if len(resp.Data) == 0 {
		return true
	}

	maxCreatedUTC := h.lastCommentCreated
	newestCommentUTC := int64(0)
	processedComments := int64(0)
	for _, comment := range resp.Data {
		if comment.ID == "" {
			continue
		}
		processedComments++
		if comment.CreatedUTC > newestCommentUTC {
			newestCommentUTC = comment.CreatedUTC
		}
		if comment.CreatedUTC > maxCreatedUTC {
			maxCreatedUTC = comment.CreatedUTC
		}

		for _, sub := range h.subscriptions {
			subMatches, err := sub.Matches(comment.Body, comment.Subreddit)
			if err != nil {
				h.logger.Error("failed to check match", "error", err, "comment_id", comment.ID)
				continue
			}
			if !subMatches {
				continue
			}

			match, err := h.makeCommentMatch(comment, sub)
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
	h.commentsTotal += processedComments
	h.commentsWindow += processedComments
	h.lastCommentCreated = maxCreatedUTC

	processingMs := time.Since(processingStart).Milliseconds()
	lagSeconds := int64(0)
	if newestCommentUTC > 0 {
		lagSeconds = time.Now().Unix() - newestCommentUTC
	}
	h.logger.Info("processed comments", "count", processedComments, "matches", len(matches), "request_ms", requestMs, "processing_ms", processingMs, "lag_seconds", lagSeconds)
	return true
}

func (h *ArcticShiftPoller) logThroughput() {
	elapsed := time.Since(h.windowStart).Minutes()
	if elapsed <= 0 {
		return
	}

	postsPerMinute := float64(h.postsWindow) / elapsed
	commentsPerMinute := float64(h.commentsWindow) / elapsed

	h.logger.Info(
		"arcticshift throughput",
		"posts_total", h.postsTotal,
		"comments_total", h.commentsTotal,
		"posts_per_min", fmt.Sprintf("%.1f", postsPerMinute),
		"comments_per_min", fmt.Sprintf("%.1f", commentsPerMinute),
	)

	h.postsWindow = 0
	h.commentsWindow = 0
	h.windowStart = time.Now()
}

func (h *ArcticShiftPoller) fetchArcticShift(url string, dest any) (int64, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("User-Agent", "feedgrep")
	start := time.Now()
	resp, err := h.client.Do(req)
	requestMs := time.Since(start).Milliseconds()
	if err != nil {
		return requestMs, fmt.Errorf("(%dms) %w", requestMs, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return requestMs, fmt.Errorf("status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return requestMs, err
	}

	return requestMs, nil
}

func (h *ArcticShiftPoller) makePostMatch(post models.ArcticShiftPost, sub keywordSubscription) (data.Match, error) {
	permalink := buildArcticShiftPostPermalink(post.Subreddit, post.ID)
	redditData := data.RedditData{
		Keyword:   sub.keyword,
		Subreddit: post.Subreddit,
		Author:    post.Author,
		Title:     post.Title,
		Body:      post.Selftext,
		IsComment: false,
		Permalink: permalink,
	}
	matchHash := buildMatchHash(sub.userID, sub.id, enums.SourceArcticShift, permalink)
	return data.NewMatch(
		sub.userID,
		sub.id,
		enums.SourceArcticShift,
		matchHash,
		redditData,
	)
}

func (h *ArcticShiftPoller) makeCommentMatch(comment models.ArcticShiftComment, sub keywordSubscription) (data.Match, error) {
	permalink := buildArcticShiftCommentPermalink(comment.Subreddit, comment.LinkID, comment.ID)
	redditData := data.RedditData{
		Keyword:   sub.keyword,
		Subreddit: comment.Subreddit,
		Author:    comment.Author,
		Title:     "",
		Body:      comment.Body,
		IsComment: true,
		Permalink: permalink,
	}
	matchHash := buildMatchHash(sub.userID, sub.id, enums.SourceArcticShift, permalink)
	return data.NewMatch(
		sub.userID,
		sub.id,
		enums.SourceArcticShift,
		matchHash,
		redditData,
	)
}

func buildArcticShiftPostPermalink(subreddit, postID string) string {
	if subreddit == "" || postID == "" {
		return ""
	}
	return fmt.Sprintf("/r/%s/comments/%s", subreddit, postID)
}

func buildArcticShiftCommentPermalink(subreddit, linkID, commentID string) string {
	if subreddit == "" || commentID == "" {
		return ""
	}
	postID := strings.TrimPrefix(linkID, "t3_")
	if postID == "" {
		postID = linkID
	}
	if postID == "" {
		return ""
	}
	return fmt.Sprintf("/r/%s/comments/%s/_/%s", subreddit, postID, commentID)
}

func (h *ArcticShiftPoller) loadKeywords() {
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

func buildMatchHash(userID uuid.UUID, keywordID int, source enums.Source, url string) string {
	input := fmt.Sprintf("%s:%d:%s:%s", userID.String(), keywordID, source, url)
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
