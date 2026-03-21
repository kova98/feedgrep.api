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
	Reddit   *RedditFilters   `json:"reddit,omitempty"`
	Language *LanguageFilters `json:"language,omitempty"`
}

func ToDataFilters(filters KeywordFilters) data.KeywordFilters {
	out := data.KeywordFilters{}

	if filters.Reddit != nil {
		out.Reddit = &data.RedditFilters{
			Subreddits:        filters.Reddit.Subreddits,
			ExcludeSubreddits: filters.Reddit.ExcludeSubreddits,
		}
	}

	if filters.Language != nil {
		out.Language = &data.LanguageFilters{
			Languages:        filters.Language.Languages,
			ExcludeLanguages: filters.Language.ExcludeLanguages,
		}
	}

	return out
}

func FromDataFilters(filters data.KeywordFilters) KeywordFilters {
	out := KeywordFilters{}

	if filters.Reddit != nil {
		out.Reddit = &RedditFilters{
			Subreddits:        filters.Reddit.Subreddits,
			ExcludeSubreddits: filters.Reddit.ExcludeSubreddits,
		}
	}

	if filters.Language != nil {
		out.Language = &LanguageFilters{
			Languages:        filters.Language.Languages,
			ExcludeLanguages: filters.Language.ExcludeLanguages,
		}
	}

	return out
}

type RedditFilters struct {
	Subreddits        []string `json:"subreddits,omitempty"`
	ExcludeSubreddits []string `json:"excludeSubreddits,omitempty"`
}

type LanguageFilters struct {
	Languages        []string `json:"languages,omitempty"`
	ExcludeLanguages []string `json:"excludeLanguages,omitempty"`
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
