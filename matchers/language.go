package matchers

import (
	"errors"
	"strings"

	"github.com/kova98/feedgrep.api/data"
	"github.com/pemistahl/lingua-go"
)

var ErrConflictingLanguageFilters = errors.New("cannot have both include and exclude language filters")
var languageDetector = lingua.NewLanguageDetectorBuilder().FromAllLanguages().Build()

func MatchesLanguage(f data.LanguageFilters, text string) (bool, error) {
	if len(f.Languages) == 0 && len(f.ExcludeLanguages) == 0 {
		return true, nil
	}

	if len(f.Languages) > 0 && len(f.ExcludeLanguages) > 0 {
		return false, ErrConflictingLanguageFilters
	}

	textLanguage, exists := languageDetector.DetectLanguageOf(text)
	if !exists {
		return len(f.Languages) == 0, nil
	}

	if len(f.ExcludeLanguages) > 0 {
		for _, excluded := range f.ExcludeLanguages {
			if matchesLanguageFilter(textLanguage, excluded) {
				return false, nil
			}
		}
		return true, nil
	}

	if len(f.Languages) > 0 {
		for _, included := range f.Languages {
			if matchesLanguageFilter(textLanguage, included) {
				return true, nil
			}
		}
		return false, nil
	}

	return true, nil
}

func matchesLanguageFilter(language lingua.Language, filter string) bool {
	if filter == "" {
		return false
	}

	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return false
	}

	name := strings.ToLower(language.String())
	if filter == name {
		return true
	}

	if code := strings.ToLower(language.IsoCode639_1().String()); filter == code {
		return true
	}

	if code := strings.ToLower(language.IsoCode639_3().String()); filter == code {
		return true
	}

	return false
}
