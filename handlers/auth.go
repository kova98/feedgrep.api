package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/google/uuid"

	"github.com/kova98/feedgrep.api/config"
	"github.com/kova98/feedgrep.api/data"
)

type AuthHandler struct {
	keycloak *gocloak.GoCloak
	clientId string
	token    string
	secret   string
	realm    string
}

func NewAuthHandler(keycloak *gocloak.GoCloak) *AuthHandler {
	return &AuthHandler{
		keycloak: keycloak,
		secret:   config.Config.KeycloakClientSecret,
		realm:    config.Config.KeycloakRealm,
		clientId: config.Config.KeycloakClientID,
	}
}

func (h *AuthHandler) StartTokenTicker() {
	err := h.refreshApiToken()
	if err != nil {
		slog.Error("Failed to refresh API token on startup", "error", err)
		return
	}

	ticker := time.NewTicker(4*time.Minute + 30*time.Second)
	for range ticker.C {
		err := h.refreshApiToken()
		if err != nil {
			slog.Error("Failed to refresh API token", "error", err)
			continue
		}
	}
}

func (h *AuthHandler) refreshApiToken() error {
	res, err := h.keycloak.LoginClient(context.Background(), h.clientId, h.secret, h.realm)
	if err != nil {
		return err
	}
	if res.AccessToken == "" {
		return errors.New("refresh api token: access token is empty")
	}
	h.token = res.AccessToken

	return nil
}

func (h *AuthHandler) GetUser(ctx context.Context, keyHeader, authHeader string) Result {
	var userInfo gocloak.UserInfo
	if authHeader != "" {
		res := h.getUserFromAuthHeader(ctx, authHeader)
		if res.Code != http.StatusOK {
			return res
		}
		userInfo = res.Body.(gocloak.UserInfo)
	} else {
		return Unauthorized("Missing authorization header")
	}

	// If preferred_username is empty, use the part before the @ in the email
	name := *userInfo.PreferredUsername
	if name == "" {
		name = strings.Split(*userInfo.Email, "@")[0]
	}

	id, err := uuid.Parse(*userInfo.Sub)
	if err != nil {
		slog.Error("Failed to parse user ID from Keycloak", "sub", userInfo.Sub, "error", err)
		return InternalError(err, "Failed to parse user ID from Keycloak")
	}

	user := data.User{
		ID:          id,
		Name:        name,
		DisplayName: *userInfo.Name,
		Email:       *userInfo.Email,
	}

	if userInfo.Picture != nil {
		user.Avatar = *userInfo.Picture
	}

	return Ok(user)
}

func (h *AuthHandler) getUserFromAuthHeader(ctx context.Context, authHeader string) Result {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return Unauthorized("Invalid authorization header format")
	}
	authHeader = strings.TrimPrefix(authHeader, "Bearer ")

	// Validate the token
	_, _, err := h.keycloak.DecodeAccessToken(ctx, authHeader, h.realm)
	if err != nil {
		return Unauthorized("Invalid token")
	}

	userInfo, err := h.keycloak.GetUserInfo(ctx, authHeader, h.realm)
	if err != nil {
		return InternalError(err, "Failed to get user info")
	}

	if userInfo == nil {
		return Unauthorized("User not found")
	}

	return Ok(*userInfo)
}
