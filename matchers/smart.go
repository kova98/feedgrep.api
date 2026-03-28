package matchers

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kova98/feedgrep.api/data"
	"github.com/pemistahl/lingua-go"
)

type SmartInput struct {
	Title     string
	Body      string
	Subreddit string
}

func MatchesSmart(filter data.SmartFilter, input SmartInput) (bool, error) {
	if !matchesScopeList(filter.Scope.Subreddits, input.Subreddit) {
		return false, nil
	}

	text := strings.TrimSpace(strings.TrimSpace(input.Title) + "\n" + strings.TrimSpace(input.Body))
	if !matchesLanguageScope(filter.Scope.Language, text) {
		return false, nil
	}

	candidateMatched, err := evaluateSmartRule(filter.Candidate, input)
	if err != nil {
		return false, err
	}
	if !candidateMatched {
		return false, nil
	}

	score := 0
	for _, signal := range filter.Signals {
		matched, err := evaluateSmartRule(data.SmartRule{
			Where:     signal.Where,
			Condition: signal.Condition,
		}, input)
		if err != nil {
			return false, err
		}
		if matched {
			score += signal.Weight
		}
	}

	return score >= filter.Thresholds.AcceptMinScore, nil
}

func matchesLanguageScope(scope data.SmartScopeList, text string) bool {
	if len(scope.Include) == 0 && len(scope.Exclude) == 0 {
		return true
	}
	if strings.TrimSpace(text) == "" {
		return len(scope.Include) == 0
	}

	detected, err := detectLanguage(text)
	if err != nil {
		return len(scope.Include) == 0
	}

	if len(scope.Include) > 0 {
		includeMatch := false
		for _, item := range scope.Include {
			if matchesLanguageFilter(detected, item) {
				includeMatch = true
				break
			}
		}
		if !includeMatch {
			return false
		}
	}

	for _, item := range scope.Exclude {
		if matchesLanguageFilter(detected, item) {
			return false
		}
	}

	return true
}

func detectLanguage(text string) (lingua.Language, error) {
	textLanguage, exists := languageDetector.DetectLanguageOf(text)
	if !exists {
		return lingua.English, fmt.Errorf("language undetected")
	}
	return textLanguage, nil
}

func matchesScopeList(scope data.SmartScopeList, value string) bool {
	normalized := normalizeSmartValue(value)
	if normalized == "" {
		return len(scope.Include) == 0
	}

	if len(scope.Include) > 0 && !containsNormalized(scope.Include, normalized) {
		return false
	}
	if len(scope.Exclude) > 0 && containsNormalized(scope.Exclude, normalized) {
		return false
	}
	return true
}

func containsNormalized(items []string, target string) bool {
	for _, item := range items {
		if normalizeSmartValue(item) == target {
			return true
		}
	}
	return false
}

func normalizeSmartValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func evaluateSmartRule(rule data.SmartRule, input SmartInput) (bool, error) {
	if isEmptySmartCondition(rule.Condition) {
		return false, nil
	}

	fields := rule.Where
	if len(fields) == 0 {
		fields = []string{"title", "body"}
	}

	return evaluateSmartCondition(rule.Condition, fields, input)
}

func evaluateSmartCondition(condition data.SmartCondition, fields []string, input SmartInput) (bool, error) {
	if len(condition.Any) > 0 {
		for _, child := range condition.Any {
			matched, err := evaluateSmartCondition(child, fields, input)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}

	if len(condition.All) > 0 {
		for _, child := range condition.All {
			matched, err := evaluateSmartCondition(child, fields, input)
			if err != nil {
				return false, err
			}
			if !matched {
				return false, nil
			}
		}
		return true, nil
	}

	if len(condition.AnyPhrase) > 0 {
		for _, field := range fields {
			value := strings.ToLower(fieldValue(field, input))
			if value == "" {
				continue
			}
			for _, phrase := range condition.AnyPhrase {
				if strings.Contains(value, strings.ToLower(strings.TrimSpace(phrase))) {
					return true, nil
				}
			}
		}
	}

	if len(condition.Regex) > 0 {
		for _, field := range fields {
			value := fieldValue(field, input)
			if value == "" {
				continue
			}
			for _, pattern := range condition.Regex {
				re, err := regexp.Compile("(?i)" + pattern)
				if err != nil {
					return false, err
				}
				if re.MatchString(value) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func fieldValue(field string, input SmartInput) string {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "title":
		return input.Title
	case "body":
		return input.Body
	case "subreddit":
		return input.Subreddit
	default:
		return ""
	}
}

func isEmptySmartCondition(condition data.SmartCondition) bool {
	return len(condition.Any) == 0 &&
		len(condition.All) == 0 &&
		len(condition.AnyPhrase) == 0 &&
		len(condition.Regex) == 0
}
