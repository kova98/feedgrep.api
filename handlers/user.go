package handlers

import (
	"net/http"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/data/repos"
)

type UserHandler struct {
	userRepo *repos.UserRepo
}

func NewUserHandler(repo *repos.UserRepo) *UserHandler {
	return &UserHandler{
		userRepo: repo,
	}
}

func (h UserHandler) InitializeUser(w http.ResponseWriter, r *http.Request) Result {
	user := r.Context().Value("user").(data.User)
	exists, err := h.userRepo.GetUserByID(user.ID)
	if err != nil {
		return InternalError(err, "initialize user: get user")
	}
	if exists != nil {
		return Ok(map[string]interface{}{"id": user.ID})
	}

	id, err := h.userRepo.InsertUser(user)
	if err != nil {
		return InternalError(err, "initialize user: insert user")
	}

	return Created(id)
}
