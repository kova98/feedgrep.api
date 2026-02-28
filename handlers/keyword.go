package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/data/repos"
	"github.com/kova98/feedgrep.api/enums"
	"github.com/kova98/feedgrep.api/models"
)

type KeywordHandler struct {
	repo      *repos.KeywordRepo
	matchRepo *repos.MatchRepo
}

func NewKeywordHandler(repo *repos.KeywordRepo, matchRepo *repos.MatchRepo) *KeywordHandler {
	return &KeywordHandler{
		repo:      repo,
		matchRepo: matchRepo,
	}
}

func (h *KeywordHandler) CreateKeyword(w http.ResponseWriter, r *http.Request) Result {
	user := r.Context().Value("user").(data.User)

	var req models.CreateKeywordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid request.")
	}

	normalized := strings.ToLower(strings.TrimSpace(req.Keyword))
	if normalized == "" {
		return BadRequest("Keyword is required.")
	}

	if len(normalized) < 4 || len(normalized) > 50 {
		return BadRequest("Keyword must be between 4 and 50 characters.")
	}

	if req.MatchMode == enums.MatchModeInvalid {
		return BadRequest("Invalid match mode.")
	}

	keyword := data.Keyword{
		UserID:    user.ID,
		Keyword:   normalized,
		MatchMode: req.MatchMode,
		Active:    true,
	}
	if req.Filters != nil {
		keyword.Filters = models.ToDataFilters(*req.Filters)
	}

	id, err := h.repo.CreateKeyword(keyword)
	if err != nil {
		return InternalError(err, "create keyword: ")
	}

	return Created(id)
}

func (h *KeywordHandler) GetKeywords(w http.ResponseWriter, r *http.Request) Result {
	user := r.Context().Value("user").(data.User)

	keywords, err := h.repo.GetKeywordsByUserID(user.ID)
	if err != nil {
		return InternalError(err, "get keywords: ")
	}

	res := &models.GetKeywordsResponse{Keywords: make([]models.Keyword, 0)}
	for _, k := range keywords {
		filters := models.FromDataFilters(k.Filters)
		res.Keywords = append(res.Keywords, models.Keyword{
			ID:        k.ID,
			UserID:    k.UserID,
			Keyword:   k.Keyword,
			Active:    k.Active,
			MatchMode: k.MatchMode,
			Filters:   &filters,
			HitCount:  k.HitCount,
		})
	}

	return Ok(res)
}

func (h *KeywordHandler) GetKeyword(w http.ResponseWriter, r *http.Request) Result {
	user := r.Context().Value("user").(data.User)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return BadRequest("Invalid keyword ID.")
	}

	keyword, err := h.repo.GetKeywordByID(id, user.ID)
	if err != nil {
		return InternalError(err, "get keyword: ")
	}
	if keyword == nil {
		return NotFound("Keyword not found.")
	}

	filters := models.FromDataFilters(keyword.Filters)
	res := models.Keyword{
		ID:        keyword.ID,
		UserID:    keyword.UserID,
		Keyword:   keyword.Keyword,
		Active:    keyword.Active,
		MatchMode: keyword.MatchMode,
		Filters:   &filters,
	}

	return Ok(res)
}

func (h *KeywordHandler) UpdateKeyword(w http.ResponseWriter, r *http.Request) Result {
	user := r.Context().Value("user").(data.User)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return BadRequest("Invalid keyword ID.")
	}

	var req models.UpdateKeywordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid request.")
	}

	normalized := strings.ToLower(strings.TrimSpace(req.Keyword))
	if normalized == "" {
		return BadRequest("Keyword is required.")
	}

	if len(normalized) < 4 || len(normalized) > 50 {
		return BadRequest("Keyword must be between 4 and 50 characters.")
	}

	if req.MatchMode == enums.MatchModeInvalid {
		return BadRequest("Invalid match mode.")
	}

	keyword := data.Keyword{
		ID:        id,
		UserID:    user.ID,
		Keyword:   normalized,
		MatchMode: req.MatchMode,
		Active:    req.Active,
	}
	if req.Filters != nil {
		keyword.Filters = models.ToDataFilters(*req.Filters)
	}

	if err := h.repo.UpdateKeyword(keyword); err != nil {
		return InternalError(err, "update keyword: ")
	}

	return Ok(nil)
}

func (h *KeywordHandler) DeleteKeyword(w http.ResponseWriter, r *http.Request) Result {
	user := r.Context().Value("user").(data.User)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return BadRequest("Invalid keyword ID.")
	}

	err = h.repo.DeleteKeyword(id, user.ID)
	if err != nil {
		return InternalError(err, "delete keyword: ")
	}

	return Ok(nil)
}

func (h *KeywordHandler) GetKeywordMatchedSubreddits(w http.ResponseWriter, r *http.Request) Result {
	user := r.Context().Value("user").(data.User)

	idStr := r.PathValue("id")
	keywordID, err := strconv.Atoi(idStr)
	if err != nil {
		return BadRequest("Invalid keyword ID.")
	}

	keyword, err := h.repo.GetKeywordByID(keywordID, user.ID)
	if err != nil {
		return InternalError(err, "get keyword: ")
	}
	if keyword == nil {
		return NotFound("Keyword not found.")
	}

	matches, err := h.matchRepo.GetMatchedSubredditsByKeyword(user.ID, keywordID, 50)
	if err != nil {
		return InternalError(err, "get keyword matched subreddits: ")
	}

	out := models.GetKeywordMatchedSubredditsResponse{
		Matches: make([]models.MatchedSubreddit, 0, len(matches)),
	}
	for _, m := range matches {
		out.Matches = append(out.Matches, models.MatchedSubreddit{
			Subreddit:     m.Subreddit,
			LastMatchedAt: m.LastMatchedAt,
			MatchCount:    m.MatchCount,
		})
	}

	return Ok(out)
}
