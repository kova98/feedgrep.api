package models

type CreateHelloRequest struct {
	Message string `json:"message"`
}

type HelloResponse struct {
	User    string `json:"user"`
	Message string `json:"message"`
}
