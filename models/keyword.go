package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/enums"
)

type CreateKeywordRequest struct {
	Keyword   string          `json:"keyword"`
	MatchMode enums.MatchMode `json:"matchMode,omitempty"`
	Filters   *KeywordFilters `json:"filters,omitempty"`
}

type UpdateKeywordRequest struct {
	Keyword   string          `json:"keyword"`
	Active    bool            `json:"active"`
	MatchMode enums.MatchMode `json:"matchMode,omitempty"`
	Filters   *KeywordFilters `json:"filters,omitempty"`
}

type KeywordFilters struct {
	Reddit *RedditFilters `json:"reddit,omitempty"`
}

func ToDataFilters(filters KeywordFilters) data.KeywordFilters {
	if filters.Reddit == nil {
		return data.KeywordFilters{}
	}
	return data.KeywordFilters{
		Reddit: &data.RedditFilters{
			Subreddits:        filters.Reddit.Subreddits,
			ExcludeSubreddits: filters.Reddit.ExcludeSubreddits,
		},
	}
}

func FromDataFilters(filters data.KeywordFilters) KeywordFilters {
	if filters.Reddit == nil {
		return KeywordFilters{}
	}
	return KeywordFilters{
		Reddit: &RedditFilters{
			Subreddits:        filters.Reddit.Subreddits,
			ExcludeSubreddits: filters.Reddit.ExcludeSubreddits,
		},
	}
}

type RedditFilters struct {
	Subreddits        []string `json:"subreddits,omitempty"`
	ExcludeSubreddits []string `json:"excludeSubreddits,omitempty"`
}

type Keyword struct {
	ID        int             `json:"id"`
	UserID    uuid.UUID       `json:"userId"`
	Keyword   string          `json:"keyword"`
	Active    bool            `json:"active"`
	MatchMode enums.MatchMode `json:"matchMode"`
	Filters   *KeywordFilters `json:"filters,omitempty"`
	HitCount  int             `json:"hitCount"`
}

type GetKeywordsResponse struct {
	Keywords []Keyword `json:"keywords"`
}

type MatchedSubreddit struct {
	Subreddit     string    `json:"subreddit"`
	LastMatchedAt time.Time `json:"lastMatchedAt"`
	MatchCount    int       `json:"matchCount"`
}

type GetKeywordMatchedSubredditsResponse struct {
	Matches []MatchedSubreddit `json:"matches"`
}
