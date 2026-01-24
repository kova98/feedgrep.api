package handlers

import (
	"errors"
	"net/http"
)

type Handler func(http.ResponseWriter, *http.Request) Result

type Result struct {
	Error error
	Code  int
	Body  interface{}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type CreatedResponse struct {
	ID interface{} `json:"id"`
}

func BadRequest(message string) Result {
	return Result{
		Code: http.StatusBadRequest,
		Body: ErrorResponse{message},
	}
}

func InternalError(error error, message string) Result {
	return Result{
		Error: errors.Join(errors.New(message), error),
		Code:  http.StatusInternalServerError,
	}
}

func NotFound(message string) Result {
	return Result{
		Code: http.StatusNotFound,
		Body: ErrorResponse{message},
	}
}

func Ok(body interface{}) Result {
	return Result{
		Code: http.StatusOK,
		Body: body,
	}
}

func Created(id interface{}) Result {
	return Result{
		Code: http.StatusCreated,
		Body: CreatedResponse{id},
	}
}

func Unauthorized(message string) Result {
	return Result{
		Code: http.StatusUnauthorized,
		Body: ErrorResponse{message},
	}
}
