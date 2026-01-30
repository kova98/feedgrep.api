package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/data/repos"
	"github.com/kova98/feedgrep.api/models"
)

type MatchHandler struct {
	repo *repos.MatchRepo
}

func NewMatchHandler(repo *repos.MatchRepo) *MatchHandler {
	return &MatchHandler{repo}
}

func (h *MatchHandler) GetMatches(w http.ResponseWriter, r *http.Request) Result {
	user := r.Context().Value("user").(data.User)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 20
	offset := (page - 1) * perPage

	matches, total, err := h.repo.GetMatchesByUserID(user.ID, perPage, offset)
	if err != nil {
		return InternalError(err, "get matches")
	}

	res := models.GetMatchesResponse{
		Matches: make([]models.Match, 0, len(matches)),
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}

	for _, m := range matches {
		var redditData data.RedditData
		_ = json.Unmarshal(m.DataRaw, &redditData)

		res.Matches = append(res.Matches, models.Match{
			ID:        m.ID,
			Keyword:   m.Keyword,
			Source:    string(m.Source),
			CreatedAt: m.CreatedAt,
			Data: models.RedditData{
				Subreddit: redditData.Subreddit,
				Author:    redditData.Author,
				Title:     redditData.Title,
				Body:      redditData.Body,
				Permalink: redditData.Permalink,
				IsComment: redditData.IsComment,
			},
		})
	}

	return Ok(res)
}
