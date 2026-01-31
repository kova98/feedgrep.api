package matchers

import (
	"strings"
	"unicode"
)

// MatchesWholeWord returns true if the keyword appears as a complete word in the text.
// Word boundaries are defined by non-alphanumeric characters or start/end of string.
func MatchesWholeWord(text, keyword string) bool {
	idx := 0
	for {
		pos := strings.Index(text[idx:], keyword)
		if pos == -1 {
			return false
		}
		pos += idx

		// Check left boundary
		leftOk := pos == 0 || !isWordChar(rune(text[pos-1]))

		// Check right boundary
		endPos := pos + len(keyword)
		rightOk := endPos == len(text) || !isWordChar(rune(text[endPos]))

		if leftOk && rightOk {
			return true
		}

		idx = pos + 1
		if idx >= len(text) {
			return false
		}
	}
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func MatchesPartially(text, keyword string) bool {
	return strings.Contains(text, keyword)
}
