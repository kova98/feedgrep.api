package matchers

import (
	"testing"

	"github.com/kova98/feedgrep.api/data"
	"github.com/stretchr/testify/assert"
)

func TestMatchesSmart(t *testing.T) {
	filter := data.SmartFilter{
		Version: "smart/v1",
		Scope: data.SmartScope{
			Language: data.SmartScopeList{Include: []string{"en"}},
		},
		Candidate: data.SmartRule{
			Where: []string{"title", "body"},
			Condition: data.SmartCondition{
				Any: []data.SmartCondition{
					{
						All: []data.SmartCondition{
							{AnyPhrase: []string{"open source", "opensource", "oss", "self-hosted"}},
							{AnyPhrase: []string{"alternative", "replacement", "equivalent"}},
						},
					},
					{
						AnyPhrase: []string{"open source alternative", "self-hosted alternative"},
					},
				},
			},
		},
		Signals: []data.SmartSignal{
			{
				Name:   "Strong title phrase",
				Weight: 40,
				Where:  []string{"title"},
				Condition: data.SmartCondition{
					AnyPhrase: []string{"open source alternative", "self-hosted alternative"},
				},
			},
			{
				Name:   "Request language",
				Weight: 20,
				Where:  []string{"title", "body"},
				Condition: data.SmartCondition{
					AnyPhrase: []string{"looking for", "is there", "can anyone recommend"},
				},
			},
			{
				Name:   "Announcement language",
				Weight: -40,
				Where:  []string{"title", "body"},
				Condition: data.SmartCondition{
					AnyPhrase: []string{"we built", "launching", "introducing"},
				},
			},
		},
		Thresholds: data.SmartThresholds{AcceptMinScore: 50},
	}

	t.Run("it accepts a request that satisfies the candidate and clears the score threshold", func(t *testing.T) {
		matched, err := MatchesSmart(filter, SmartInput{
			Title: "Looking for an open source alternative to Notion?",
			Body:  "Can anyone recommend something self-hosted for note taking?",
		})
		assert.NoError(t, err)
		assert.True(t, matched)
	})

	t.Run("it rejects content that never satisfies the candidate condition", func(t *testing.T) {
		matched, err := MatchesSmart(filter, SmartInput{
			Title: "Weekly roundup of note taking tools",
			Body:  "A comparison of popular productivity apps.",
		})
		assert.NoError(t, err)
		assert.False(t, matched)
	})

	t.Run("it rejects candidates whose negative signals pull the score below the threshold", func(t *testing.T) {
		matched, err := MatchesSmart(filter, SmartInput{
			Title: "We built an open source alternative to Notion",
			Body:  "Launching today for teams who want a self-hosted wiki.",
		})
		assert.NoError(t, err)
		assert.False(t, matched)
	})

	t.Run("it applies subreddit scope filters before scoring", func(t *testing.T) {
		scoped := filter
		scoped.Scope.Subreddits = data.SmartScopeList{Include: []string{"selfhosted"}}

		matched, err := MatchesSmart(scoped, SmartInput{
			Title:     "Looking for an open source alternative to Notion?",
			Body:      "Can anyone recommend something self-hosted for note taking?",
			Subreddit: "productivity",
		})
		assert.NoError(t, err)
		assert.False(t, matched)
	})
}
