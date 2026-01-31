package sources

import (
	"testing"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/enums"
	"github.com/stretchr/testify/assert"
)

func newTestSubscription(keyword string, filters *data.RedditFilters) keywordSubscription {
	return keywordSubscription{
		id:        1,
		userID:    uuid.New(),
		keyword:   keyword,
		matchMode: enums.MatchModeBroad,
		filters:   filters,
	}
}

func TestMatches_KeywordOnly(t *testing.T) {
	sub := newTestSubscription("golang", nil)

	match, err := sub.Matches("i love golang", "programming")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = sub.Matches("golang is great", "any")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = sub.Matches("i love rust", "programming")
	assert.NoError(t, err)
	assert.False(t, match)
}

func TestMatches_KeywordCaseInsensitive(t *testing.T) {
	sub := newTestSubscription("golang", nil)

	match, err := sub.Matches("golang is great", "any")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = sub.Matches("GOLANG is great", "any")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = sub.Matches("GoLang Is Great", "any")
	assert.NoError(t, err)
	assert.True(t, match)
}

func TestMatches_WithSubredditIncludeFilter(t *testing.T) {
	sub := newTestSubscription("golang", &data.RedditFilters{
		Subreddits: []string{"programming", "golang"},
	})

	match, err := sub.Matches("golang rocks", "programming")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = sub.Matches("golang rocks", "golang")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = sub.Matches("golang rocks", "funny")
	assert.NoError(t, err)
	assert.False(t, match)
}

func TestMatches_WithSubredditExcludeFilter(t *testing.T) {
	sub := newTestSubscription("golang", &data.RedditFilters{
		ExcludeSubreddits: []string{"circlejerk"},
	})

	match, err := sub.Matches("golang rocks", "programming")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = sub.Matches("golang rocks", "circlejerk")
	assert.NoError(t, err)
	assert.False(t, match)
}

func TestMatches_KeywordMustMatchEvenWithFilters(t *testing.T) {
	sub := newTestSubscription("golang", &data.RedditFilters{
		Subreddits: []string{"programming"},
	})

	match, err := sub.Matches("rust is cool", "programming")
	assert.NoError(t, err)
	assert.False(t, match, "keyword must match even if subreddit matches")
}

func TestMatches_InvalidMatchMode(t *testing.T) {
	sub := keywordSubscription{
		id:        1,
		keyword:   "golang",
		matchMode: enums.MatchModeInvalid,
	}

	_, err := sub.Matches("golang rocks", "programming")
	assert.Error(t, err)
}
