package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kova98/feedgrep.api/models"
)

const smartFilterPrompt = `Convert the intent description into a valid smart/v1 JSON config.

Output rules:
- Output valid JSON only.
- Do not wrap it in markdown.
- Do not add any explanation.
- Use only the fields and operators described below.
- Use camelCase keys exactly as shown.
- Do not include unknown keys.
- Use integer weights only.
- Prefer multi-word phrases over single generic words.

Schema:
{
  "version": "smart/v1",
  "name": "string",
  "description": "string",
  "scope": {
    "language": {
      "include": ["string"],
      "exclude": ["string"]
    },
    "subreddits": {
      "include": ["string"],
      "exclude": ["string"]
    }
  },
  "candidate": {
    "where": ["title", "body"],
    "condition": {
      "any": [Condition],
      "all": [Condition],
      "anyPhrase": ["string"],
      "regex": ["string"]
    }
  },
  "signals": [
    {
      "name": "string",
      "weight": 0,
      "where": ["title", "body"],
      "condition": {
        "any": [Condition],
        "all": [Condition],
        "anyPhrase": ["string"],
        "regex": ["string"]
      }
    }
  ],
  "thresholds": {
    "acceptMinScore": 0
  }
}

Condition rules:
- candidate.condition is the fast first-pass retrieval query.
- candidate must optimize for recall, not precision.
- candidate should be simple and index-friendly.
- Prefer anyPhrase, any, and all.
- Avoid regex unless clearly necessary.
- signals are for scoring, not retrieval.

Candidate design rules:
- candidate should usually capture broad domain concepts, category phrases, workflow phrases, product names, entity names, or related terminology.
- candidate should not usually require strong intent phrases like frustration, request, complaint, or recommendation language.
- Do not make candidate so strict that only near-perfect phrasing is retrieved.
- Prefer broad domain retrieval first, then let signals decide relevance.
- Use all in candidate only when multiple concepts are truly essential to the intent.
- Prefer any in candidate when several alternative phrasings or domain entry points are possible.

Signal design rules:
- Use positive signals for the actual intent:
  - requests
  - pain points
  - complaints
  - feature requests
  - unmet needs
  - comparisons
  - recommendation seeking
  - buying intent
  - switching intent
  depending on the user’s description.
- Use negative signals for likely noise:
  - promotions
  - announcements
  - tutorials
  - listicles
  - generic discussion
  - unrelated adjacent topics
  depending on the user’s description.
- Strong title signals should weigh more than weak body signals.
- Broad domain phrases should not be enough on their own to create a match.
- Intent should mostly be expressed in signals, not in candidate.

Quality rules:
- Prefer phrase-level cues over broad single words.
- Avoid vague single-word signals like like, need, recommend, problem, or issue unless part of a phrase.
- Prefer phrases like looking for, wish there was, frustrated with, feature request, would love, alternative to, we built, top 10 when relevant.
- Use subreddits only when the intent clearly implies them.
- If language is unspecified, default to English only when reasonable.
- The config should be broad enough to retrieve plausible candidates, then selective enough in signals to reduce noise.

Important balancing rule:
- If forced to choose, make candidate broader and let signals and acceptMinScore do the filtering.
- Do not put all precision into candidate.
- Do not require both domain language and explicit intent language in candidate unless the intent is extremely narrow and high precision is more important than recall.

Return exactly one smart/v1 JSON object.`

type SmartFilterGenerator struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

type openAIResponsesRequest struct {
	Model           string                 `json:"model"`
	Input           string                 `json:"input"`
	MaxOutputTokens int                    `json:"max_output_tokens,omitempty"`
	Temperature     float64                `json:"temperature,omitempty"`
	Text            openAIResponseTextSpec `json:"text"`
}

type openAIResponseTextSpec struct {
	Format openAIResponseFormat `json:"format"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIResponsesResponse struct {
	OutputText string               `json:"output_text"`
	Output     []openAIOutputItem   `json:"output"`
	Error      *openAIErrorEnvelope `json:"error"`
}

type openAIOutputItem struct {
	Content []openAIOutputContent `json:"content"`
}

type openAIOutputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIErrorEnvelope struct {
	Message string `json:"message"`
}

func NewSmartFilterGenerator(apiKey, model string) *SmartFilterGenerator {
	return &SmartFilterGenerator{
		apiKey: strings.TrimSpace(apiKey),
		model:  strings.TrimSpace(model),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (g *SmartFilterGenerator) Generate(ctx context.Context, name, intent string) (models.SmartFilter, error) {
	prompt := buildSmartFilterPrompt(name, intent)
	reqBody := openAIResponsesRequest{
		Model:           g.model,
		Input:           prompt,
		MaxOutputTokens: 4000,
		Temperature:     0.2,
		Text: openAIResponseTextSpec{
			Format: openAIResponseFormat{Type: "json_object"},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return models.SmartFilter{}, fmt.Errorf("marshal openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(bodyBytes))
	if err != nil {
		return models.SmartFilter{}, fmt.Errorf("create openai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return models.SmartFilter{}, fmt.Errorf("call openai: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return models.SmartFilter{}, fmt.Errorf("read openai response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return models.SmartFilter{}, fmt.Errorf("openai error: %s", strings.TrimSpace(string(respBody)))
	}

	var parsedResp openAIResponsesResponse
	if err := json.Unmarshal(respBody, &parsedResp); err != nil {
		return models.SmartFilter{}, fmt.Errorf("decode openai response: %w", err)
	}
	if parsedResp.Error != nil && parsedResp.Error.Message != "" {
		return models.SmartFilter{}, fmt.Errorf("openai error: %s", parsedResp.Error.Message)
	}

	outputText := extractOpenAIOutputText(parsedResp)
	if strings.TrimSpace(outputText) == "" {
		return models.SmartFilter{}, fmt.Errorf("openai returned empty output")
	}

	var filter models.SmartFilter
	if err := json.Unmarshal([]byte(outputText), &filter); err != nil {
		return models.SmartFilter{}, fmt.Errorf("decode generated filter: %w", err)
	}

	if err := normalizeSmartFilter(&filter, name, intent); err != nil {
		return models.SmartFilter{}, err
	}

	return filter, nil
}

func buildSmartFilterPrompt(name, intent string) string {
	intent = strings.TrimSpace(intent)
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Smart filter"
	}

	return fmt.Sprintf("%s\n\nFilter name:\n%s\n\nIntent description:\n%s", smartFilterPrompt, name, intent)
}

func extractOpenAIOutputText(resp openAIResponsesResponse) string {
	if strings.TrimSpace(resp.OutputText) != "" {
		return resp.OutputText
	}

	var builder strings.Builder
	for _, item := range resp.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" || content.Type == "text" {
				if builder.Len() > 0 {
					builder.WriteByte('\n')
				}
				builder.WriteString(content.Text)
			}
		}
	}
	return builder.String()
}

func normalizeSmartFilter(filter *models.SmartFilter, name, intent string) error {
	if filter == nil {
		return fmt.Errorf("generated filter is empty")
	}
	if strings.TrimSpace(filter.Version) == "" {
		filter.Version = "smart/v1"
	}
	if strings.TrimSpace(filter.Name) == "" {
		filter.Name = strings.TrimSpace(name)
	}
	if strings.TrimSpace(filter.Description) == "" {
		filter.Description = strings.TrimSpace(intent)
	}
	if len(filter.Scope.Language.Include) == 0 && len(filter.Scope.Language.Exclude) == 0 {
		filter.Scope.Language.Include = []string{"en"}
	}
	if filter.Scope.Language.Exclude == nil {
		filter.Scope.Language.Exclude = []string{}
	}
	if filter.Scope.Subreddits.Include == nil {
		filter.Scope.Subreddits.Include = []string{}
	}
	if filter.Scope.Subreddits.Exclude == nil {
		filter.Scope.Subreddits.Exclude = []string{}
	}
	if len(filter.Candidate.Where) == 0 {
		filter.Candidate.Where = []string{"title", "body"}
	}
	if isEmptySmartCondition(filter.Candidate.Condition) {
		return fmt.Errorf("generated filter is missing candidate.condition")
	}
	if filter.Signals == nil {
		filter.Signals = []models.SmartSignal{}
	}
	if filter.Thresholds.AcceptMinScore == 0 {
		filter.Thresholds.AcceptMinScore = 40
	}

	return nil
}

func isEmptySmartCondition(condition models.SmartCondition) bool {
	return len(condition.Any) == 0 &&
		len(condition.All) == 0 &&
		len(condition.AnyPhrase) == 0 &&
		len(condition.Regex) == 0
}
