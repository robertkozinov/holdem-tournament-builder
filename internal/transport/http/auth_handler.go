package http

import (
	"context"
	"encoding/json"
	"errors"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"holdem-tournament-builder/internal/transport/response"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const sessionCookieName = "session_token"
const maxAuthRequestBytes = 4 << 10

type AuthService interface {
	Register(ctx context.Context, username, password string, now time.Time) (service.AuthResult, error)
	Login(ctx context.Context, username, password string, now time.Time) (service.AuthResult, error)
	Authenticate(ctx context.Context, rawToken string, now time.Time) (*domain.User, error)
	Logout(ctx context.Context, rawToken string) error
}

type AuthHandler struct {
	service      AuthService
	secureCookie bool
}

func NewAuthHandler(service AuthService, secureCookie bool) *AuthHandler {
	return &AuthHandler{service: service, secureCookie: secureCookie}
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authUserResponse struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
}

func decodeAuthRequest(w http.ResponseWriter, r *http.Request) (authRequest, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return authRequest{}, errors.New("invalid content type")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req authRequest
	if err := decoder.Decode(&req); err != nil {
		return authRequest{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return authRequest{}, errors.New("invalid request body")
	}
	return req, nil
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrInvalidUsername):
		response.WriteError(w, http.StatusBadRequest, "invalid username")
	case errors.Is(err, app.ErrInvalidPassword):
		response.WriteError(w, http.StatusBadRequest, "invalid password")
	case errors.Is(err, app.ErrUsernameAlreadyExists):
		response.WriteError(w, http.StatusConflict, "username already exists")
	case errors.Is(err, app.ErrInvalidCredentials):
		response.WriteError(w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, app.ErrUnauthenticated):
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
	default:
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	req, err := decodeAuthRequest(w, r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	result, err := h.service.Register(r.Context(), req.Username, req.Password, time.Now())
	if err != nil {
		writeAuthError(w, err)
		return
	}

	h.setSessionCookie(w, result.Token, result.ExpiresAt)
	response.WriteJSON(w, http.StatusCreated, authUserResponse{ID: result.UserID, Username: result.Username})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	req, err := decodeAuthRequest(w, r)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	result, err := h.service.Login(r.Context(), req.Username, req.Password, time.Now())
	if err != nil {
		writeAuthError(w, err)
		return
	}

	h.setSessionCookie(w, result.Token, result.ExpiresAt)
	response.WriteJSON(w, http.StatusOK, authUserResponse{ID: result.UserID, Username: result.Username})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var rawToken string
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		rawToken = cookie.Value
	}

	if err := h.service.Logout(r.Context(), rawToken); err != nil {
		writeAuthError(w, err)
		return
	}

	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	response.WriteJSON(w, http.StatusOK, authUserResponse{ID: user.ID, Username: user.Username})
}
