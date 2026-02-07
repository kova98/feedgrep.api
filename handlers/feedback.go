package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/models"
	"github.com/kova98/feedgrep.api/notifiers"
)

type FeedbackHandler struct {
	mailer *notifiers.Mailer
}

func NewFeedbackHandler(mailer *notifiers.Mailer) *FeedbackHandler {
	return &FeedbackHandler{mailer}
}

type feedbackRequest struct {
	Feedback string `json:"feedback"`
}

func (h *FeedbackHandler) SubmitFeedback(w http.ResponseWriter, r *http.Request) Result {
	user := r.Context().Value("user").(data.User)

	var req feedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid request.")
	}

	if req.Feedback == "" {
		return BadRequest("Feedback is required.")
	}

	email := models.Email{
		To:      "roko@feedgrep.com",
		Subject: "feedgrep feedback",
		Body:    fmt.Sprintf("<strong>From:</strong> %s<br/><br/>%s", user.Email, req.Feedback),
	}

	if err := h.mailer.Send(email); err != nil {
		return InternalError(err, "send feedback email: ")
	}

	return Ok(nil)
}
