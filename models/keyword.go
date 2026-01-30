package models

import (
	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/data"
)

type CreateKeywordRequest struct {
	Keyword string `json:"keyword"`
}

type UpdateKeywordRequest struct {
	Keyword string          `json:"keyword"`
	Active  bool            `json:"active"`
	Filters *KeywordFilters `json:"filters,omitempty"`
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
	ID      int             `json:"id"`
	UserID  uuid.UUID       `json:"userId"`
	Keyword string          `json:"keyword"`
	Active  bool            `json:"active"`
	Filters *KeywordFilters `json:"filters,omitempty"`
}

type GetKeywordsResponse struct {
	Keywords []Keyword `json:"keywords"`
}
