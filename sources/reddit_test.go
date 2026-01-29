package sources

import (
	"testing"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/data"
	"github.com/stretchr/testify/assert"
)

func newTestSubscription(keyword string, filters *data.RedditFilters) keywordSubscription {
	return keywordSubscription{
		id:      1,
		userID:  uuid.New(),
		keyword: keyword,
		filters: filters,
	}
}

func TestMatches_KeywordOnly(t *testing.T) {
	sub := newTestSubscription("golang", nil)

	assert.True(t, sub.Matches("i love golang", "programming"))
	assert.True(t, sub.Matches("golang is great", "any"))
	assert.False(t, sub.Matches("i love rust", "programming"))
}

func TestMatches_KeywordCaseInsensitive(t *testing.T) {
	sub := newTestSubscription("golang", nil)

	assert.True(t, sub.Matches("golang is great", "any"))
	assert.True(t, sub.Matches("GOLANG is great", "any"))
	assert.True(t, sub.Matches("GoLang Is Great", "any"))
}

func TestMatches_WithSubredditIncludeFilter(t *testing.T) {
	sub := newTestSubscription("golang", &data.RedditFilters{
		Subreddits: []string{"programming", "golang"},
	})

	assert.True(t, sub.Matches("golang rocks", "programming"))
	assert.True(t, sub.Matches("golang rocks", "golang"))
	assert.False(t, sub.Matches("golang rocks", "funny"))
}

func TestMatches_WithSubredditExcludeFilter(t *testing.T) {
	sub := newTestSubscription("golang", &data.RedditFilters{
		ExcludeSubreddits: []string{"circlejerk"},
	})

	assert.True(t, sub.Matches("golang rocks", "programming"))
	assert.False(t, sub.Matches("golang rocks", "circlejerk"))
}

func TestMatches_KeywordMustMatchEvenWithFilters(t *testing.T) {
	sub := newTestSubscription("golang", &data.RedditFilters{
		Subreddits: []string{"programming"},
	})

	assert.False(t, sub.Matches("rust is cool", "programming"), "keyword must match even if subreddit matches")
}
