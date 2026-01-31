package matchers

import (
	"errors"
	"strings"

	"github.com/kova98/feedgrep.api/data"
)

var ErrConflictingFilters = errors.New("cannot have both include and exclude subreddit filters")

func MatchesSubreddit(f data.RedditFilters, subreddit string) (bool, error) {
	if len(f.Subreddits) > 0 && len(f.ExcludeSubreddits) > 0 {
		return false, ErrConflictingFilters
	}

	if len(f.ExcludeSubreddits) > 0 {
		for _, excluded := range f.ExcludeSubreddits {
			if strings.EqualFold(excluded, subreddit) {
				return false, nil
			}
		}
		return true, nil
	}

	if len(f.Subreddits) > 0 {
		for _, included := range f.Subreddits {
			if strings.EqualFold(included, subreddit) {
				return true, nil
			}
		}
		return false, nil
	}

	return true, nil
}
