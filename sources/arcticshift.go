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
	"github.com/kova98/feedgrep.api/monitor"
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
	am          *monitor.ArcticShiftMonitor
	km          *monitor.KeywordMonitor
	client      *http.Client

	subscriptions       []keywordSubscription
	postPollInterval    time.Duration
	commentPollInterval time.Duration
	lastPostCreated     int64
	lastCommentCreated  int64
}

func NewArcticShiftPoller(logger *slog.Logger, keywordRepo *repos.KeywordRepo, matchRepo *repos.MatchRepo, arcticShiftMonitor *monitor.ArcticShiftMonitor, keywordMonitor *monitor.KeywordMonitor) *ArcticShiftPoller {
	interval := time.Duration(config.Config.PostPollIntervalMs) * time.Millisecond

	return &ArcticShiftPoller{
		logger:              logger,
		keywordRepo:         keywordRepo,
		matchRepo:           matchRepo,
		am:                  arcticShiftMonitor,
		km:                  keywordMonitor,
		client:              &http.Client{Timeout: 15 * time.Second},
		postPollInterval:    interval,
		commentPollInterval: interval,
	}
}

func (h *ArcticShiftPoller) StartPolling(ctx context.Context) {
	h.logger.Info("starting arcticshift polling",
		"post_interval", h.postPollInterval.Seconds(),
		"comment_interval", h.commentPollInterval.Seconds())

	h.loadKeywords()

	ticker := time.NewTicker(h.postPollInterval)
	keywordTicker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	defer keywordTicker.Stop()

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
		h.am.PostRequestError(time.Duration(requestMs) * time.Millisecond)
		h.logger.Info("poll posts", "request_ms", requestMs, "error", truncateError(err))
		return false
	}

	if len(resp.Data) == 0 {
		h.am.PostRequest(time.Duration(requestMs) * time.Millisecond)
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

		for _, sub := range h.subscriptions {
			matchStart := time.Now()
			subMatches, smartResult, err := sub.Matches(post.Title, post.Selftext, post.Subreddit)
			h.am.PostMatchEvaluation(string(sub.matchMode), matchStart)
			if err != nil {
				h.logger.Error("failed to check match", "error", err, "post_id", post.ID)
				continue
			}
			h.logSmartMatchResult("post", post.ID, sub, smartResult)
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
	h.am.PostBatch(processedPosts, time.Duration(requestMs)*time.Millisecond, processingStart, newestPostUTC)
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
		h.am.CommentRequestError(time.Duration(requestMs) * time.Millisecond)
		h.logger.Debug("poll comments", "request_ms", requestMs, "error", truncateError(err))
		return false
	}

	if len(resp.Data) == 0 {
		h.am.CommentRequest(time.Duration(requestMs) * time.Millisecond)
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
			matchStart := time.Now()
			subMatches, smartResult, err := sub.Matches("", comment.Body, comment.Subreddit)
			h.am.CommentMatchEvaluation(string(sub.matchMode), matchStart)
			if err != nil {
				h.logger.Error("failed to check match", "error", err, "comment_id", comment.ID)
				continue
			}
			h.logSmartMatchResult("comment", comment.ID, sub, smartResult)
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
	h.lastCommentCreated = maxCreatedUTC

	h.am.CommentBatch(processedComments, time.Duration(requestMs)*time.Millisecond, processingStart, newestCommentUTC)
	return true
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
			filters:   keyword.Filters,
		})
	}

	h.subscriptions = active
	h.km.Active(len(h.subscriptions))
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
	filters   data.KeywordFilters
}

func (h *ArcticShiftPoller) logSmartMatchResult(kind, itemID string, sub keywordSubscription, result *matchers.SmartMatchResult) {
	if result == nil {
		return
	}
	if !result.CandidateMatched {
		return
	}

	h.logger.Debug(
		"smart match evaluation",
		"keyword_id", sub.id,
		"matched", result.Matched,
		"score", result.Score,
		"matched_signals", result.MatchedSignals,
		"signal_details", result.SignalDetails,
		"rejected_by", result.RejectedBy,
	)
}

func (s *keywordSubscription) Matches(title, body, subreddit string) (bool, *matchers.SmartMatchResult, error) {
	text := strings.TrimSpace(strings.TrimSpace(title) + "\n" + strings.TrimSpace(body))
	textLower := strings.ToLower(text)

	if s.matchMode == enums.MatchModeInvalid {
		return false, nil, errors.New(string("invalid match mode: " + s.matchMode))
	}

	switch s.matchMode {
	case enums.MatchModeExact:
		if !matchers.MatchesWholeWord(textLower, s.keyword) {
			return false, nil, nil
		}
	case enums.MatchModeBroad:
		if !matchers.MatchesPartially(textLower, s.keyword) {
			return false, nil, nil
		}
	case enums.MatchModeSmart:
		if s.filters.Smart == nil {
			return false, nil, errors.New("smart match mode requires a smart filter")
		}
		result, err := matchers.EvaluateSmart(*s.filters.Smart, matchers.SmartInput{
			Title:     title,
			Body:      body,
			Subreddit: subreddit,
		})
		if err != nil {
			return false, nil, err
		}
		if !result.Matched {
			return false, &result, nil
		}
		return true, &result, nil
	default:
		return false, nil, errors.New(string("invalid match mode: " + s.matchMode))
	}

	if s.filters.Reddit != nil {
		match, err := matchers.MatchesSubreddit(*s.filters.Reddit, subreddit)
		if err != nil {
			return false, nil, err
		}
		if !match {
			return false, nil, nil
		}
	}

	if s.filters.Language != nil {
		match, err := matchers.MatchesLanguage(*s.filters.Language, text)
		if err != nil {
			return false, nil, err
		}
		if !match {
			return false, nil, nil
		}
	}

	return true, nil, nil
}
