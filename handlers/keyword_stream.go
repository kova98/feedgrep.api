package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/enums"
	"github.com/kova98/feedgrep.api/matchers"
)

const historicalProgressEvery = 25

type searchStreamRequest struct {
	Query string `json:"query"`
}

type searchStreamHit struct {
	Rank      int     `json:"rank"`
	Score     float64 `json:"score"`
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	YearMonth string  `json:"year_month"`
	Subreddit string  `json:"subreddit"`
	Author    string  `json:"author"`
	CreatedAt int64   `json:"created_at"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
}

type searchStreamEnd struct {
	SearchedIndexes int `json:"searchedIndexes"`
	HitCount        int `json:"hitCount"`
}

type browserProgressEvent struct {
	Processed int `json:"processed"`
	Matched   int `json:"matched"`
}

type browserMatchEvent struct {
	ID             string                            `json:"id"`
	Kind           string                            `json:"kind"`
	YearMonth      string                            `json:"yearMonth"`
	Subreddit      string                            `json:"subreddit"`
	Author         string                            `json:"author"`
	CreatedAt      int64                             `json:"createdAt"`
	Title          string                            `json:"title"`
	Body           string                            `json:"body"`
	RetrievalScore float64                           `json:"retrievalScore"`
	SmartScore     int                               `json:"smartScore"`
	MatchedSignals []string                          `json:"matchedSignals"`
	SignalDetails  []matchers.SmartSignalMatchDetail `json:"signalDetails"`
}

type browserEndEvent struct {
	Processed       int    `json:"processed"`
	Matched         int    `json:"matched"`
	SearchedIndexes int    `json:"searchedIndexes"`
	CandidateHits   int    `json:"candidateHits"`
	Query           string `json:"query"`
}

func (h *KeywordHandler) StreamHistoricalSmartMatches(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(data.User)

	idStr := r.PathValue("id")
	keywordID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid keyword ID.", http.StatusBadRequest)
		return
	}

	keyword, err := h.repo.GetKeywordByID(keywordID, user.ID)
	if err != nil {
		http.Error(w, "Failed to load keyword.", http.StatusInternalServerError)
		return
	}
	if keyword == nil {
		http.Error(w, "Keyword not found.", http.StatusNotFound)
		return
	}
	if keyword.MatchMode != enums.MatchModeSmart || keyword.Filters.Smart == nil {
		http.Error(w, "Historical streaming requires a smart keyword.", http.StatusBadRequest)
		return
	}

	query, err := compileSmartCandidateQuery(keyword.Filters.Smart.Candidate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported.", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	upstreamReqBody, err := json.Marshal(searchStreamRequest{Query: query})
	if err != nil {
		http.Error(w, "Failed to create search request.", http.StatusInternalServerError)
		return
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.searchURL+"/stream", bytes.NewReader(upstreamReqBody))
	if err != nil {
		http.Error(w, "Failed to create upstream request.", http.StatusInternalServerError)
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Accept", "text/event-stream")

	upstreamResp, err := http.DefaultClient.Do(upstreamReq)
	if err != nil {
		http.Error(w, "Failed to contact search service.", http.StatusBadGateway)
		return
	}
	defer upstreamResp.Body.Close()

	if upstreamResp.StatusCode != http.StatusOK {
		http.Error(w, "Search service failed.", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	processed := 0
	matched := 0

	writeSSE(w, "start", map[string]string{"query": query})
	flusher.Flush()

	err = scanSSE(ctx, upstreamResp.Body, func(eventType, payload string) error {
		switch eventType {
		case "hit":
			var hit searchStreamHit
			if err := json.Unmarshal([]byte(payload), &hit); err != nil {
				return err
			}
			processed++

			result, err := matchers.EvaluateSmart(*keyword.Filters.Smart, matchers.SmartInput{
				Title:     hit.Title,
				Body:      hit.Body,
				Subreddit: hit.Subreddit,
			})
			if err != nil {
				return err
			}
			if result.Matched {
				matched++
				writeSSE(w, "match", browserMatchEvent{
					ID:             hit.ID,
					Kind:           hit.Kind,
					YearMonth:      hit.YearMonth,
					Subreddit:      hit.Subreddit,
					Author:         hit.Author,
					CreatedAt:      hit.CreatedAt,
					Title:          hit.Title,
					Body:           hit.Body,
					RetrievalScore: hit.Score,
					SmartScore:     result.Score,
					MatchedSignals: result.MatchedSignals,
					SignalDetails:  result.SignalDetails,
				})
				flusher.Flush()
			}

			if processed%historicalProgressEvery == 0 {
				writeSSE(w, "progress", browserProgressEvent{
					Processed: processed,
					Matched:   matched,
				})
				flusher.Flush()
			}
		case "end":
			var end searchStreamEnd
			if err := json.Unmarshal([]byte(payload), &end); err != nil {
				return err
			}
			writeSSE(w, "end", browserEndEvent{
				Processed:       processed,
				Matched:         matched,
				SearchedIndexes: end.SearchedIndexes,
				CandidateHits:   end.HitCount,
				Query:           query,
			})
			flusher.Flush()
		case "error":
			writeSSE(w, "error", map[string]string{"error": payload})
			flusher.Flush()
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		writeSSE(w, "error", map[string]string{"error": err.Error()})
		flusher.Flush()
	}
}

func compileSmartCandidateQuery(rule data.SmartRule) (string, error) {
	fields := rule.Where
	if len(fields) == 0 {
		fields = []string{"title", "body"}
	}

	query, err := compileSmartCondition(rule.Condition, fields)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("smart candidate cannot be empty")
	}
	return query, nil
}

func compileSmartCondition(condition data.SmartCondition, fields []string) (string, error) {
	switch {
	case len(condition.Any) > 0:
		parts := make([]string, 0, len(condition.Any))
		for _, child := range condition.Any {
			part, err := compileSmartCondition(child, fields)
			if err != nil {
				return "", err
			}
			if part != "" {
				parts = append(parts, part)
			}
		}
		return joinQueryParts(parts, "OR"), nil
	case len(condition.All) > 0:
		parts := make([]string, 0, len(condition.All))
		for _, child := range condition.All {
			part, err := compileSmartCondition(child, fields)
			if err != nil {
				return "", err
			}
			if part != "" {
				parts = append(parts, part)
			}
		}
		return joinQueryParts(parts, "AND"), nil
	case len(condition.AnyPhrase) > 0:
		fieldCount := len(fields)
		if fieldCount == 0 {
			fieldCount = 2
		}
		parts := make([]string, 0, len(condition.AnyPhrase)*fieldCount)
		targetFields := fields
		if len(targetFields) == 0 {
			targetFields = []string{"title", "body"}
		}
		for _, phrase := range condition.AnyPhrase {
			escaped := quoteQueryPhrase(phrase)
			for _, field := range targetFields {
				switch field {
				case "title", "body", "subreddit":
					parts = append(parts, fmt.Sprintf(`%s:%s`, field, escaped))
				default:
					return "", fmt.Errorf("unsupported smart candidate field %q", field)
				}
			}
		}
		return joinQueryParts(parts, "OR"), nil
	case len(condition.Regex) > 0:
		return "", fmt.Errorf("smart historical search does not support regex candidates")
	default:
		return "", nil
	}
}

func joinQueryParts(parts []string, op string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " "+op+" ") + ")"
}

func quoteQueryPhrase(value string) string {
	escaped := strings.ReplaceAll(strings.TrimSpace(value), `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func scanSSE(ctx context.Context, body io.ReadCloser, onEvent func(eventType, payload string) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var eventType string
	var dataLines []string

	emit := func() error {
		if len(dataLines) == 0 {
			eventType = ""
			return nil
		}
		kind := eventType
		if kind == "" {
			kind = "message"
		}
		payload := strings.Join(dataLines, "\n")
		eventType = ""
		dataLines = dataLines[:0]
		return onEvent(kind, payload)
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			if err := emit(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return emit()
}

func writeSSE(w http.ResponseWriter, event string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", body)
}
