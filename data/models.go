package data

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/enums"
)

type KeywordNotification struct {
	ID         int             `db:"id"`
	UserID     uuid.UUID       `db:"user_id"`
	Keyword    string          `db:"keyword"`
	MatchMode  enums.MatchMode `db:"match_mode"`
	Email      string          `db:"email"`
	FiltersRaw json.RawMessage `db:"filters"`
	Filters    KeywordFilters  `db:"-"`
}

type KeywordWithStats struct {
	Keyword
	HitCount      int        `db:"hit_count"`
	UnseenCount   int        `db:"unseen_count"`
	LastMatchedAt *time.Time `db:"last_matched_at"`
}

type MatchWithKeyword struct {
	Match
	Keyword string `db:"keyword"`
}

type MatchedSubredditSummary struct {
	Subreddit     string    `db:"subreddit"`
	LastMatchedAt time.Time `db:"last_matched_at"`
	MatchCount    int       `db:"match_count"`
}

type KeywordDailyMatchCountRow struct {
	Day   time.Time `db:"day"`
	Count int       `db:"count"`
}
