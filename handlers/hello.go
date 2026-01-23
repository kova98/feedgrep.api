package handlers

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/data"
	"log/slog"
	"net/http"

	"github.com/kova98/feedgrep.api/models"
)

type HelloHandler struct {
	hello models.HelloResponse
}

func NewHelloHandler() *HelloHandler {
	return &HelloHandler{}
}

func (h *HelloHandler) GetHello(w http.ResponseWriter, r *http.Request) Result {
	return Ok(h.hello)
}

func (h *HelloHandler) PostHello(w http.ResponseWriter, r *http.Request) Result {
	var req models.CreateHelloRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid request.")
	}

	if req.Message == "" {
		return BadRequest("Message is required")
	}

	slog.Info("received hello message", "message", req.Message)

	user := r.Context().Value("user").(data.User)

	h.hello = models.HelloResponse{
		Message: req.Message,
		User:    user.Name,
	}

	return Created(uuid.New())
}
