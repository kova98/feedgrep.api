package matchers

import (
	"testing"

	"github.com/kova98/feedgrep.api/data"
	"github.com/stretchr/testify/assert"
)

func TestMatchesLanguage(t *testing.T) {
	t.Run("it matches any text when no language filters are provided", func(t *testing.T) {
		filters := data.LanguageFilters{}

		match, err := MatchesLanguage(filters, "This is a simple english sentence.")
		assert.NoError(t, err)
		assert.True(t, match)
	})

	t.Run("it matches only texts written in the included languages", func(t *testing.T) {
		filters := data.LanguageFilters{
			Languages: []string{"en", "german"},
		}

		match, err := MatchesLanguage(filters, "This is a simple english sentence.")
		assert.NoError(t, err)
		assert.True(t, match)

		match, err = MatchesLanguage(filters, "To je slovenska poved.")
		assert.NoError(t, err)
		assert.False(t, match)
	})

	t.Run("it excludes texts written in blocked languages", func(t *testing.T) {
		filters := data.LanguageFilters{
			ExcludeLanguages: []string{"en"},
		}

		match, err := MatchesLanguage(filters, "This is a simple english sentence.")
		assert.NoError(t, err)
		assert.False(t, match)

		match, err = MatchesLanguage(filters, "Das ist ein deutscher Satz.")
		assert.NoError(t, err)
		assert.True(t, match)
	})

	t.Run("it returns an error when include and exclude filters are both provided", func(t *testing.T) {
		filters := data.LanguageFilters{
			Languages:        []string{"en"},
			ExcludeLanguages: []string{"de"},
		}

		_, err := MatchesLanguage(filters, "This is a simple english sentence.")
		assert.ErrorIs(t, err, ErrConflictingLanguageFilters)
	})

	t.Run("it treats configured language names and codes case insensitively", func(t *testing.T) {
		includeFilters := data.LanguageFilters{Languages: []string{"ENGLISH"}}
		excludeFilters := data.LanguageFilters{ExcludeLanguages: []string{"EN"}}

		match, err := MatchesLanguage(includeFilters, "This is a simple english sentence.")
		assert.NoError(t, err)
		assert.True(t, match)

		match, err = MatchesLanguage(excludeFilters, "This is a simple english sentence.")
		assert.NoError(t, err)
		assert.False(t, match)
	})
}
