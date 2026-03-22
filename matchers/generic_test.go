package matchers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesWholeWord(t *testing.T) {
	t.Run("it matches complete words at the beginning middle and end of the text", func(t *testing.T) {
		assert.True(t, MatchesWholeWord("hello world", "hello"))
		assert.True(t, MatchesWholeWord("hello world", "world"))
		assert.True(t, MatchesWholeWord("hello world ", "world"))
		assert.True(t, MatchesWholeWord("hello", "hello"))
	})

	t.Run("it does not match when the query is only part of a larger word", func(t *testing.T) {
		assert.False(t, MatchesWholeWord("application", "app"))
		assert.False(t, MatchesWholeWord("unhappy", "happy"))
		assert.False(t, MatchesWholeWord("goodbye", "good"))
	})

	t.Run("it treats punctuation as valid word boundaries", func(t *testing.T) {
		assert.True(t, MatchesWholeWord("hello, world!", "hello"))
		assert.True(t, MatchesWholeWord("hello, world!", "world"))
		assert.True(t, MatchesWholeWord("(app)", "app"))
		assert.True(t, MatchesWholeWord("check this app.", "app"))
	})

	t.Run("it matches whole words even when partial matches also appear elsewhere", func(t *testing.T) {
		assert.True(t, MatchesWholeWord("the application has an app", "app"))
		assert.False(t, MatchesWholeWord("application apps", "app"))
	})

	t.Run("it handles edge cases around empty input and boundary positions", func(t *testing.T) {
		assert.True(t, MatchesWholeWord("app", "app"))
		assert.False(t, MatchesWholeWord("", "app"))
		assert.True(t, MatchesWholeWord("app at start", "app"))
		assert.True(t, MatchesWholeWord("ends with app", "app"))
	})
}

func TestMatchesPartially(t *testing.T) {
	t.Run("it matches when the query appears anywhere inside the text", func(t *testing.T) {
		assert.True(t, MatchesPartially("application", "app"))
		assert.True(t, MatchesPartially("unhappy", "happy"))
		assert.True(t, MatchesPartially("hello world", "hello"))
		assert.True(t, MatchesPartially("hello world", "world"))
	})

	t.Run("it does not match when the query does not appear in the text", func(t *testing.T) {
		assert.False(t, MatchesPartially("hello", "world"))
		assert.False(t, MatchesPartially("golang", "rust"))
	})

	t.Run("it handles edge cases for empty strings", func(t *testing.T) {
		assert.True(t, MatchesPartially("app", "app"))
		assert.False(t, MatchesPartially("", "app"))
		assert.True(t, MatchesPartially("app", ""))
	})
}
