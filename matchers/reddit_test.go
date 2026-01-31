package matchers

import (
	"testing"

	"github.com/kova98/feedgrep.api/data"
	"github.com/stretchr/testify/assert"
)

func TestMatchesSubreddit_EmptyFilters(t *testing.T) {
	filters := data.RedditFilters{}

	match, err := MatchesSubreddit(filters, "anything")
	assert.NoError(t, err)
	assert.True(t, match)
}

func TestMatchesSubreddit_IncludeList(t *testing.T) {
	filters := data.RedditFilters{
		Subreddits: []string{"golang", "programming"},
	}

	match, err := MatchesSubreddit(filters, "golang")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = MatchesSubreddit(filters, "programming")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = MatchesSubreddit(filters, "funny")
	assert.NoError(t, err)
	assert.False(t, match)
}

func TestMatchesSubreddit_ExcludeList(t *testing.T) {
	filters := data.RedditFilters{
		ExcludeSubreddits: []string{"circlejerk", "test"},
	}

	match, err := MatchesSubreddit(filters, "circlejerk")
	assert.NoError(t, err)
	assert.False(t, match)

	match, err = MatchesSubreddit(filters, "test")
	assert.NoError(t, err)
	assert.False(t, match)

	match, err = MatchesSubreddit(filters, "golang")
	assert.NoError(t, err)
	assert.True(t, match)
}

func TestMatchesSubreddit_ConflictingFiltersReturnsError(t *testing.T) {
	filters := data.RedditFilters{
		Subreddits:        []string{"golang", "rust"},
		ExcludeSubreddits: []string{"circlejerk"},
	}

	_, err := MatchesSubreddit(filters, "golang")
	assert.ErrorIs(t, err, ErrConflictingFilters)
}

func TestMatchesSubreddit_CaseInsensitive(t *testing.T) {
	includeFilters := data.RedditFilters{Subreddits: []string{"GoLang"}}
	excludeFilters := data.RedditFilters{ExcludeSubreddits: []string{"GOLANG"}}

	match, err := MatchesSubreddit(includeFilters, "golang")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = MatchesSubreddit(includeFilters, "GOLANG")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = MatchesSubreddit(excludeFilters, "golang")
	assert.NoError(t, err)
	assert.False(t, match)

	match, err = MatchesSubreddit(excludeFilters, "GoLang")
	assert.NoError(t, err)
	assert.False(t, match)
}
