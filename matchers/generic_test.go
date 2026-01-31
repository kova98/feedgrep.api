package matchers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesWholeWord_ExactMatch(t *testing.T) {
	assert.True(t, MatchesWholeWord("hello world", "hello"))
	assert.True(t, MatchesWholeWord("hello world", "world"))
	assert.True(t, MatchesWholeWord("hello world ", "world"))
	assert.True(t, MatchesWholeWord("hello", "hello"))
}

func TestMatchesWholeWord_NoMatch(t *testing.T) {
	assert.False(t, MatchesWholeWord("application", "app"))
	assert.False(t, MatchesWholeWord("unhappy", "happy"))
	assert.False(t, MatchesWholeWord("goodbye", "good"))
}

func TestMatchesWholeWord_WithPunctuation(t *testing.T) {
	assert.True(t, MatchesWholeWord("hello, world!", "hello"))
	assert.True(t, MatchesWholeWord("hello, world!", "world"))
	assert.True(t, MatchesWholeWord("(app)", "app"))
	assert.True(t, MatchesWholeWord("check this app.", "app"))
}

func TestMatchesWholeWord_MultipleOccurrences(t *testing.T) {
	assert.True(t, MatchesWholeWord("the application has an app", "app"))
	assert.False(t, MatchesWholeWord("application apps", "app"))
}

func TestMatchesWholeWord_EdgeCases(t *testing.T) {
	assert.True(t, MatchesWholeWord("app", "app"))
	assert.False(t, MatchesWholeWord("", "app"))
	assert.True(t, MatchesWholeWord("app at start", "app"))
	assert.True(t, MatchesWholeWord("ends with app", "app"))
}

func TestMatchesPartially_Match(t *testing.T) {
	assert.True(t, MatchesPartially("application", "app"))
	assert.True(t, MatchesPartially("unhappy", "happy"))
	assert.True(t, MatchesPartially("hello world", "hello"))
	assert.True(t, MatchesPartially("hello world", "world"))
}

func TestMatchesPartially_NoMatch(t *testing.T) {
	assert.False(t, MatchesPartially("hello", "world"))
	assert.False(t, MatchesPartially("golang", "rust"))
}

func TestMatchesPartially_EdgeCases(t *testing.T) {
	assert.True(t, MatchesPartially("app", "app"))
	assert.False(t, MatchesPartially("", "app"))
	assert.True(t, MatchesPartially("app", ""))
}
