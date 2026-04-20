package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/google/uuid"

	"github.com/kova98/feedgrep.api/config"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/data/repos"
	"github.com/kova98/feedgrep.api/models"
	"github.com/kova98/feedgrep.api/notifiers"
)

const (
	authActionResetPassword = "reset_password"
)

type AuthHandler struct {
	keycloak  *gocloak.GoCloak
	clientId  string
	token     string
	secret    string
	realm     string
	tokenRepo *repos.AuthActionTokenRepo
	mailer    *notifiers.Mailer
}

func NewAuthHandler(keycloak *gocloak.GoCloak, tokenRepo *repos.AuthActionTokenRepo, mailer *notifiers.Mailer) *AuthHandler {
	return &AuthHandler{
		keycloak:  keycloak,
		secret:    config.Config.KeycloakClientSecret,
		realm:     config.Config.KeycloakRealm,
		clientId:  config.Config.KeycloakClientID,
		tokenRepo: tokenRepo,
		mailer:    mailer,
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

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) Result {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid login request")
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		return BadRequest("Email and password are required")
	}

	token, err := h.keycloak.Login(r.Context(), h.clientId, h.secret, h.realm, req.Email, req.Password)
	if err != nil {
		slog.Debug("Keycloak password grant failed", "email", req.Email, "error", err)
		return Unauthorized("Invalid email or password")
	}

	return Ok(models.NewTokenResponse(token))
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) Result {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid registration request")
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		return BadRequest("Email and password are required")
	}
	if len(req.Password) < 8 {
		return BadRequest("Password must be at least 8 characters")
	}

	if h.token == "" {
		return InternalError(errors.New("api token unavailable"), "register: get admin token")
	}

	credentials := []gocloak.CredentialRepresentation{
		{
			Type:      gocloak.StringP("password"),
			Value:     gocloak.StringP(req.Password),
			Temporary: gocloak.BoolP(false),
		},
	}

	_, err := h.keycloak.CreateUser(r.Context(), h.token, h.realm, gocloak.User{
		Username:      gocloak.StringP(req.Email),
		Email:         gocloak.StringP(req.Email),
		FirstName:     gocloak.StringP(defaultFirstName(req.Email)),
		Enabled:       gocloak.BoolP(true),
		EmailVerified: gocloak.BoolP(true),
		Credentials:   &credentials,
	})
	if err != nil {
		if strings.Contains(err.Error(), "409 Conflict") {
			return BadRequest("An account with this email already exists")
		}

		return InternalError(err, "Unable to create account")
	}

	return Created(models.RegisterResponse{Email: req.Email})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) Result {
	var req models.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid reset password request")
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		return BadRequest("Email is required")
	}

	if h.token == "" {
		return InternalError(errors.New("api token unavailable"), "reset password: get admin token")
	}

	params := gocloak.GetUsersParams{
		Email: &req.Email,
		Exact: gocloak.BoolP(true),
		Max:   gocloak.IntP(1),
	}
	users, err := h.keycloak.GetUsers(r.Context(), h.token, h.realm, params)
	if err != nil {
		return InternalError(err, "reset password: find user")
	}

	if len(users) == 0 || users[0] == nil {
		return Ok(map[string]bool{"ok": true})
	}

	user := users[0]
	if user.ID == nil || *user.ID == "" {
		return InternalError(errors.New("keycloak user id is empty"), "reset password: find user")
	}

	rawToken, err := h.createAuthActionToken(*user.ID, req.Email, authActionResetPassword, time.Hour)
	if err != nil {
		return InternalError(err, "reset password: create reset token")
	}

	link := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(config.Config.AppBaseURL, "/"), rawToken)
	if err := h.mailer.Send(h.mailer.PasswordResetEmail(req.Email, link)); err != nil {
		return InternalError(err, "reset password: send reset email")
	}

	return Ok(map[string]bool{"ok": true})
}

func (h *AuthHandler) ConfirmResetPassword(w http.ResponseWriter, r *http.Request) Result {
	var req models.ConfirmResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid reset password request")
	}
	if len(req.Password) < 8 {
		return BadRequest("Password must be at least 8 characters")
	}

	token, err := h.getValidAuthActionToken(req.Token, authActionResetPassword)
	if err != nil {
		return InternalError(err, "reset password: get token")
	}
	if token == nil {
		return BadRequest("Password reset link is invalid or expired")
	}

	if h.token == "" {
		return InternalError(errors.New("api token unavailable"), "reset password: get admin token")
	}

	if err := h.keycloak.SetPassword(r.Context(), h.token, token.UserID, h.realm, req.Password, false); err != nil {
		return InternalError(err, "reset password: set password")
	}

	if err := h.tokenRepo.MarkUsed(token.ID); err != nil {
		return InternalError(err, "reset password: mark token used")
	}

	return Ok(map[string]bool{"ok": true})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) Result {
	var req models.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid refresh request")
	}

	if req.RefreshToken == "" {
		return BadRequest("Refresh token is required")
	}

	token, err := h.keycloak.RefreshToken(r.Context(), req.RefreshToken, h.clientId, h.secret, h.realm)
	if err != nil {
		return Unauthorized("Session expired")
	}

	return Ok(models.NewTokenResponse(token))
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) Result {
	var req models.LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return BadRequest("Invalid logout request")
	}

	if req.RefreshToken == "" {
		return Ok(map[string]bool{"ok": true})
	}

	if err := h.keycloak.Logout(r.Context(), h.clientId, h.secret, h.realm, req.RefreshToken); err != nil {
		slog.Debug("Failed to revoke Keycloak session", "error", err)
	}

	return Ok(map[string]bool{"ok": true})
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

func defaultFirstName(email string) string {
	name := strings.Split(email, "@")[0]
	if name == "" {
		return "User"
	}

	return name
}

func (h *AuthHandler) createAuthActionToken(userID, email, action string, ttl time.Duration) (string, error) {
	rawToken, tokenHash, err := newAuthActionToken()
	if err != nil {
		return "", err
	}

	err = h.tokenRepo.Insert(data.AuthActionToken{
		ID:        uuid.New(),
		UserID:    userID,
		Email:     email,
		Action:    action,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(ttl),
	})
	if err != nil {
		return "", err
	}

	return rawToken, nil
}

func (h *AuthHandler) getValidAuthActionToken(rawToken, action string) (*data.AuthActionToken, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, nil
	}

	return h.tokenRepo.GetValid(action, hashAuthActionToken(rawToken), time.Now())
}

func newAuthActionToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	rawToken := base64.RawURLEncoding.EncodeToString(bytes)
	return rawToken, hashAuthActionToken(rawToken), nil
}

func hashAuthActionToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}
