package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/enums"
)

type CreateKeywordRequest struct {
	Keyword   string          `json:"keyword"`
	MatchMode enums.MatchMode `json:"matchMode,omitempty"`
	Filters   *KeywordFilters `json:"filters,omitempty"`
}

type UpdateKeywordRequest struct {
	Keyword   string          `json:"keyword"`
	Active    bool            `json:"active"`
	MatchMode enums.MatchMode `json:"matchMode,omitempty"`
	Filters   *KeywordFilters `json:"filters,omitempty"`
}

type KeywordFilters struct {
	Reddit   *RedditFilters   `json:"reddit,omitempty"`
	Language *LanguageFilters `json:"language,omitempty"`
	Smart    *SmartFilter     `json:"smart,omitempty"`
}

func ToDataFilters(filters KeywordFilters) data.KeywordFilters {
	out := data.KeywordFilters{}

	if filters.Reddit != nil {
		out.Reddit = &data.RedditFilters{
			Subreddits:        filters.Reddit.Subreddits,
			ExcludeSubreddits: filters.Reddit.ExcludeSubreddits,
		}
	}

	if filters.Language != nil {
		out.Language = &data.LanguageFilters{
			Languages:        filters.Language.Languages,
			ExcludeLanguages: filters.Language.ExcludeLanguages,
		}
	}

	if filters.Smart != nil {
		out.Smart = toDataSmartFilter(*filters.Smart)
	}

	return out
}

func FromDataFilters(filters data.KeywordFilters) KeywordFilters {
	out := KeywordFilters{}

	if filters.Reddit != nil {
		out.Reddit = &RedditFilters{
			Subreddits:        filters.Reddit.Subreddits,
			ExcludeSubreddits: filters.Reddit.ExcludeSubreddits,
		}
	}

	if filters.Language != nil {
		out.Language = &LanguageFilters{
			Languages:        filters.Language.Languages,
			ExcludeLanguages: filters.Language.ExcludeLanguages,
		}
	}

	if filters.Smart != nil {
		out.Smart = fromDataSmartFilter(*filters.Smart)
	}

	return out
}

type RedditFilters struct {
	Subreddits        []string `json:"subreddits,omitempty"`
	ExcludeSubreddits []string `json:"excludeSubreddits,omitempty"`
}

type LanguageFilters struct {
	Languages        []string `json:"languages,omitempty"`
	ExcludeLanguages []string `json:"excludeLanguages,omitempty"`
}

type SmartFilter struct {
	Version     string          `json:"version,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Scope       SmartScope      `json:"scope,omitempty"`
	Candidate   SmartRule       `json:"candidate"`
	Signals     []SmartSignal   `json:"signals,omitempty"`
	Thresholds  SmartThresholds `json:"thresholds,omitempty"`
}

type SmartScope struct {
	Language   SmartScopeList `json:"language,omitempty"`
	Subreddits SmartScopeList `json:"subreddits,omitempty"`
}

type SmartScopeList struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

type SmartRule struct {
	Where     []string       `json:"where,omitempty"`
	Condition SmartCondition `json:"condition"`
}

type SmartSignal struct {
	Name      string         `json:"name,omitempty"`
	Weight    int            `json:"weight"`
	Where     []string       `json:"where,omitempty"`
	Condition SmartCondition `json:"condition"`
}

type SmartCondition struct {
	Any       []SmartCondition `json:"any,omitempty"`
	All       []SmartCondition `json:"all,omitempty"`
	AnyPhrase []string         `json:"anyPhrase,omitempty"`
	Regex     []string         `json:"regex,omitempty"`
}

func (c *SmartCondition) UnmarshalJSON(data []byte) error {
	type rawCondition struct {
		Any       []json.RawMessage `json:"any,omitempty"`
		All       []json.RawMessage `json:"all,omitempty"`
		AnyPhrase json.RawMessage   `json:"anyPhrase,omitempty"`
		Regex     json.RawMessage   `json:"regex,omitempty"`
	}

	var raw rawCondition
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	any, err := parseSmartConditionChildren(raw.Any)
	if err != nil {
		return fmt.Errorf("decode any: %w", err)
	}
	all, err := parseSmartConditionChildren(raw.All)
	if err != nil {
		return fmt.Errorf("decode all: %w", err)
	}
	anyPhrase, err := parseSmartConditionStrings(raw.AnyPhrase)
	if err != nil {
		return fmt.Errorf("decode anyPhrase: %w", err)
	}
	regex, err := parseSmartConditionStrings(raw.Regex)
	if err != nil {
		return fmt.Errorf("decode regex: %w", err)
	}

	c.Any = any
	c.All = all
	c.AnyPhrase = anyPhrase
	c.Regex = regex
	return nil
}

func parseSmartConditionChildren(items []json.RawMessage) ([]SmartCondition, error) {
	if len(items) == 0 {
		return nil, nil
	}

	out := make([]SmartCondition, 0, len(items))
	for _, item := range items {
		trimmed := string(item)
		if len(trimmed) == 0 {
			continue
		}

		var phrase string
		if err := json.Unmarshal(item, &phrase); err == nil {
			phrase = normalizeGeneratedString(phrase)
			if phrase == "" {
				continue
			}
			out = append(out, SmartCondition{AnyPhrase: []string{phrase}})
			continue
		}

		var child SmartCondition
		if err := json.Unmarshal(item, &child); err != nil {
			return nil, err
		}
		out = append(out, child)
	}

	return out, nil
}

func parseSmartConditionStrings(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		single = normalizeGeneratedString(single)
		if single == "" {
			return nil, nil
		}
		return []string{single}, nil
	}

	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		out := make([]string, 0, len(many))
		for _, item := range many {
			item = normalizeGeneratedString(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out, nil
	}

	return nil, fmt.Errorf("expected string or []string")
}

func normalizeGeneratedString(value string) string {
	return strings.TrimSpace(value)
}

type SmartThresholds struct {
	AcceptMinScore int `json:"acceptMinScore,omitempty"`
}

func toDataSmartFilter(filter SmartFilter) *data.SmartFilter {
	signals := make([]data.SmartSignal, 0, len(filter.Signals))
	for _, signal := range filter.Signals {
		signals = append(signals, data.SmartSignal{
			Name:      signal.Name,
			Weight:    signal.Weight,
			Where:     append([]string(nil), signal.Where...),
			Condition: toDataSmartCondition(signal.Condition),
		})
	}

	return &data.SmartFilter{
		Version:     filter.Version,
		Name:        filter.Name,
		Description: filter.Description,
		Scope: data.SmartScope{
			Language:   toDataSmartScopeList(filter.Scope.Language),
			Subreddits: toDataSmartScopeList(filter.Scope.Subreddits),
		},
		Candidate: data.SmartRule{
			Where:     append([]string(nil), filter.Candidate.Where...),
			Condition: toDataSmartCondition(filter.Candidate.Condition),
		},
		Signals: signals,
		Thresholds: data.SmartThresholds{
			AcceptMinScore: filter.Thresholds.AcceptMinScore,
		},
	}
}

func fromDataSmartFilter(filter data.SmartFilter) *SmartFilter {
	signals := make([]SmartSignal, 0, len(filter.Signals))
	for _, signal := range filter.Signals {
		signals = append(signals, SmartSignal{
			Name:      signal.Name,
			Weight:    signal.Weight,
			Where:     append([]string(nil), signal.Where...),
			Condition: fromDataSmartCondition(signal.Condition),
		})
	}

	return &SmartFilter{
		Version:     filter.Version,
		Name:        filter.Name,
		Description: filter.Description,
		Scope: SmartScope{
			Language:   fromDataSmartScopeList(filter.Scope.Language),
			Subreddits: fromDataSmartScopeList(filter.Scope.Subreddits),
		},
		Candidate: SmartRule{
			Where:     append([]string(nil), filter.Candidate.Where...),
			Condition: fromDataSmartCondition(filter.Candidate.Condition),
		},
		Signals: signals,
		Thresholds: SmartThresholds{
			AcceptMinScore: filter.Thresholds.AcceptMinScore,
		},
	}
}

func toDataSmartScopeList(list SmartScopeList) data.SmartScopeList {
	return data.SmartScopeList{
		Include: append([]string(nil), list.Include...),
		Exclude: append([]string(nil), list.Exclude...),
	}
}

func fromDataSmartScopeList(list data.SmartScopeList) SmartScopeList {
	return SmartScopeList{
		Include: append([]string(nil), list.Include...),
		Exclude: append([]string(nil), list.Exclude...),
	}
}

func toDataSmartCondition(condition SmartCondition) data.SmartCondition {
	out := data.SmartCondition{
		AnyPhrase: append([]string(nil), condition.AnyPhrase...),
		Regex:     append([]string(nil), condition.Regex...),
	}
	if len(condition.Any) > 0 {
		out.Any = make([]data.SmartCondition, 0, len(condition.Any))
		for _, child := range condition.Any {
			out.Any = append(out.Any, toDataSmartCondition(child))
		}
	}
	if len(condition.All) > 0 {
		out.All = make([]data.SmartCondition, 0, len(condition.All))
		for _, child := range condition.All {
			out.All = append(out.All, toDataSmartCondition(child))
		}
	}
	return out
}

func fromDataSmartCondition(condition data.SmartCondition) SmartCondition {
	out := SmartCondition{
		AnyPhrase: append([]string(nil), condition.AnyPhrase...),
		Regex:     append([]string(nil), condition.Regex...),
	}
	if len(condition.Any) > 0 {
		out.Any = make([]SmartCondition, 0, len(condition.Any))
		for _, child := range condition.Any {
			out.Any = append(out.Any, fromDataSmartCondition(child))
		}
	}
	if len(condition.All) > 0 {
		out.All = make([]SmartCondition, 0, len(condition.All))
		for _, child := range condition.All {
			out.All = append(out.All, fromDataSmartCondition(child))
		}
	}
	return out
}

type Keyword struct {
	ID        int             `json:"id"`
	UserID    uuid.UUID       `json:"userId"`
	Keyword   string          `json:"keyword"`
	Active    bool            `json:"active"`
	MatchMode enums.MatchMode `json:"matchMode"`
	Filters   *KeywordFilters `json:"filters,omitempty"`
	HitCount  int             `json:"hitCount"`
}

type GetKeywordsResponse struct {
	Keywords []Keyword `json:"keywords"`
}

type MatchedSubreddit struct {
	Subreddit     string    `json:"subreddit"`
	LastMatchedAt time.Time `json:"lastMatchedAt"`
	MatchCount    int       `json:"matchCount"`
}

type GetKeywordMatchedSubredditsResponse struct {
	Matches []MatchedSubreddit `json:"matches"`
}

type GenerateSmartFilterRequest struct {
	Name   string `json:"name"`
	Intent string `json:"intent"`
}

type GenerateSmartFilterResponse struct {
	Filter SmartFilter `json:"filter"`
}
