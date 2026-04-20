package models

import "github.com/Nerzal/gocloak/v13"

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ResetPasswordRequest struct {
	Email string `json:"email"`
}

type ConfirmResetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type TokenResponse struct {
	AccessToken      string `json:"accessToken"`
	ExpiresIn        int    `json:"expiresIn"`
	RefreshToken     string `json:"refreshToken"`
	RefreshExpiresIn int    `json:"refreshExpiresIn"`
	TokenType        string `json:"tokenType"`
}

func NewTokenResponse(token *gocloak.JWT) TokenResponse {
	return TokenResponse{
		AccessToken:      token.AccessToken,
		ExpiresIn:        token.ExpiresIn,
		RefreshToken:     token.RefreshToken,
		RefreshExpiresIn: token.RefreshExpiresIn,
		TokenType:        token.TokenType,
	}
}

type RegisterResponse struct {
	Email string `json:"email"`
}
