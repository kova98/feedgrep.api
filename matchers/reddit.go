package matchers

import (
	"strings"

	"github.com/kova98/feedgrep.api/data"
)

func MatchesSubreddit(f data.RedditFilters, subreddit string) bool {
	// Check exclude list first
	for _, excluded := range f.ExcludeSubreddits {
		if strings.EqualFold(excluded, subreddit) {
			return false
		}
	}

	// If include list is empty, allow all (that weren't excluded)
	if len(f.Subreddits) == 0 {
		return true
	}

	// Check include list
	for _, included := range f.Subreddits {
		if strings.EqualFold(included, subreddit) {
			return true
		}
	}

	return false
}
