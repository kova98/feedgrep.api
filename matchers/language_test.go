package matchers

import (
	"testing"

	"github.com/kova98/feedgrep.api/data"
	"github.com/stretchr/testify/assert"
)

func TestMatchesLanguage_EmptyFilters(t *testing.T) {
	filters := data.LanguageFilters{}

	match, err := MatchesLanguage(filters, "This is a simple english sentence.")
	assert.NoError(t, err)
	assert.True(t, match)
}

func TestMatchesLanguage_IncludeList(t *testing.T) {
	filters := data.LanguageFilters{
		Languages: []string{"en", "german"},
	}

	match, err := MatchesLanguage(filters, "This is a simple english sentence.")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = MatchesLanguage(filters, "To je slovenska poved.")
	assert.NoError(t, err)
	assert.False(t, match)
}

func TestMatchesLanguage_ExcludeList(t *testing.T) {
	filters := data.LanguageFilters{
		ExcludeLanguages: []string{"en"},
	}

	match, err := MatchesLanguage(filters, "This is a simple english sentence.")
	assert.NoError(t, err)
	assert.False(t, match)

	match, err = MatchesLanguage(filters, "Das ist ein deutscher Satz.")
	assert.NoError(t, err)
	assert.True(t, match)
}

func TestMatchesLanguage_ConflictingFiltersReturnsError(t *testing.T) {
	filters := data.LanguageFilters{
		Languages:        []string{"en"},
		ExcludeLanguages: []string{"de"},
	}

	_, err := MatchesLanguage(filters, "This is a simple english sentence.")
	assert.ErrorIs(t, err, ErrConflictingLanguageFilters)
}

func TestMatchesLanguage_CaseInsensitive(t *testing.T) {
	includeFilters := data.LanguageFilters{Languages: []string{"ENGLISH"}}
	excludeFilters := data.LanguageFilters{ExcludeLanguages: []string{"EN"}}

	match, err := MatchesLanguage(includeFilters, "This is a simple english sentence.")
	assert.NoError(t, err)
	assert.True(t, match)

	match, err = MatchesLanguage(excludeFilters, "This is a simple english sentence.")
	assert.NoError(t, err)
	assert.False(t, match)
}
