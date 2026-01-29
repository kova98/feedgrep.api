package matchers

import (
	"testing"

	"github.com/kova98/feedgrep.api/data"
	"github.com/stretchr/testify/assert"
)

func TestMatchesSubreddit_EmptyFilters(t *testing.T) {
	filters := data.RedditFilters{}
	assert.True(t, MatchesSubreddit(filters, "anything"))
}

func TestMatchesSubreddit_IncludeList(t *testing.T) {
	filters := data.RedditFilters{
		Subreddits: []string{"golang", "programming"},
	}

	assert.True(t, MatchesSubreddit(filters, "golang"))
	assert.True(t, MatchesSubreddit(filters, "programming"))
	assert.False(t, MatchesSubreddit(filters, "funny"))
}

func TestMatchesSubreddit_ExcludeList(t *testing.T) {
	filters := data.RedditFilters{
		ExcludeSubreddits: []string{"circlejerk", "test"},
	}

	assert.False(t, MatchesSubreddit(filters, "circlejerk"))
	assert.False(t, MatchesSubreddit(filters, "test"))
	assert.True(t, MatchesSubreddit(filters, "golang"))
}

func TestMatchesSubreddit_ExcludeTakesPrecedence(t *testing.T) {
	filters := data.RedditFilters{
		Subreddits:        []string{"golang", "rust"},
		ExcludeSubreddits: []string{"golang"},
	}

	assert.False(t, MatchesSubreddit(filters, "golang"), "excluded should override included")
	assert.True(t, MatchesSubreddit(filters, "rust"))
}

func TestMatchesSubreddit_CaseInsensitive(t *testing.T) {
	includeFilters := data.RedditFilters{Subreddits: []string{"GoLang"}}
	excludeFilters := data.RedditFilters{ExcludeSubreddits: []string{"GOLANG"}}

	assert.True(t, MatchesSubreddit(includeFilters, "golang"))
	assert.True(t, MatchesSubreddit(includeFilters, "GOLANG"))
	assert.False(t, MatchesSubreddit(excludeFilters, "golang"))
	assert.False(t, MatchesSubreddit(excludeFilters, "GoLang"))
}
