package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/service"
	"holdem-tournament-builder/internal/transport/response"
)

type AuthService interface {
	Register(context.Context, string, string, time.Time) (domain.User, error)
	Login(context.Context, string, string, time.Time) (service.AuthResult, error)
	Authenticate(context.Context, string, time.Time) (uuid.UUID, error)
	Logout(context.Context, string) error
	CurrentUser(context.Context, uuid.UUID) (domain.User, error)
}

type CookieOptions struct{ Secure bool }

type AuthHandler struct {
	service AuthService
	cookies CookieOptions
	now     func() time.Time
	safety  *requestSafety
}

func NewAuthHandler(service AuthService, cookies CookieOptions) *AuthHandler {
	h := &AuthHandler{service: service, cookies: cookies, now: time.Now}
	h.safety = newRequestSafety(func() time.Time { return h.now() })
	return h
}

const maxAuthRequestBytes = 4 << 10

type authRequestBody struct {
	Login    *string `json:"login"`
	Password *string `json:"password"`
}

func (b *authRequestBody) UnmarshalJSON(data []byte) error {
	*b = authRequestBody{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil {
		return err
	}
	if start != json.Delim('{') {
		return app.ErrValidation
	}
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			return err
		}
		var field **string
		switch name {
		case "login":
			field = &b.Login
		case "password":
			field = &b.Password
		default:
			return app.ErrValidation
		}
		if *field != nil {
			return app.ErrValidation
		}
		if err := decoder.Decode(field); err != nil {
			return err
		}
		if *field == nil {
			return app.ErrValidation
		}
	}
	_, err = decoder.Token()
	return err
}

type authUserResponse struct {
	ID    uuid.UUID `json:"id"`
	Login string    `json:"login"`
}

func decodeAuthRequest(w http.ResponseWriter, r *http.Request) (*authRequestBody, bool) {
	req, ok := decodeLimitedRequest[authRequestBody](w, r, maxAuthRequestBytes)
	if !ok {
		return nil, false
	}
	if req.Login == nil || req.Password == nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request")
		return nil, false
	}
	if r.Context().Err() != nil {
		response.WriteError(w, http.StatusServiceUnavailable, "request unavailable")
		return nil, false
	}
	return req, true
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	req, ok := decodeAuthRequest(w, r)
	if !ok {
		return
	}
	user, err := h.service.Register(r.Context(), *req.Login, *req.Password, h.now())
	if err != nil {
		handleAuthError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusCreated, authUserResponse{ID: user.ID, Login: user.Login})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	req, ok := decodeAuthRequest(w, r)
	if !ok {
		return
	}
	result, err := h.service.Login(r.Context(), *req.Login, *req.Password, h.now())
	if err != nil {
		handleAuthError(w, err)
		return
	}
	maxAge := int(math.Ceil(result.ExpiresAt.Sub(h.now()).Seconds()))
	cookie := h.sessionCookie(result.Token)
	cookie.Expires = result.ExpiresAt
	cookie.MaxAge = maxAge
	if result.UserID == uuid.Nil || result.Token == "" || maxAge <= 0 || cookie.Valid() != nil {
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	http.SetCookie(w, cookie)
	response.WriteJSON(w, http.StatusOK, authUserResponse{ID: result.UserID, Login: result.Login})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	token, ok := h.sessionToken(r)
	if !ok {
		handleAuthError(w, app.ErrUnauthenticated)
		return
	}
	if token != "" {
		if err := h.service.Logout(r.Context(), token); err != nil {
			handleAuthError(w, err)
			return
		}
	}
	cookie := h.sessionCookie("")
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0).UTC()
	http.SetCookie(w, cookie)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, err := app.UserID(r.Context())
	if err != nil {
		handleAuthError(w, err)
		return
	}
	user, err := h.service.CurrentUser(r.Context(), id)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	if user.ID != id {
		handleAuthError(w, app.ErrUnauthenticated)
		return
	}
	response.WriteJSON(w, http.StatusOK, authUserResponse{ID: user.ID, Login: user.Login})
}

func (h *AuthHandler) cookieName() string {
	if h.cookies.Secure {
		return "__Host-holdem_session"
	}
	return "holdem_session"
}

func (h *AuthHandler) sessionCookie(token string) *http.Cookie {
	return &http.Cookie{Name: h.cookieName(), Value: token, Path: "/", HttpOnly: true, Secure: h.cookies.Secure, SameSite: http.SameSiteLaxMode}
}

func (h *AuthHandler) sessionToken(r *http.Request) (string, bool) {
	name := h.cookieName()
	var sessionField string
	count := 0
	for _, line := range r.Header.Values("Cookie") {
		for len(line) > 0 {
			field, rest, _ := strings.Cut(line, ";")
			line = rest
			field = strings.TrimSpace(field)
			fieldName, _, _ := strings.Cut(field, "=")
			if strings.TrimSpace(fieldName) != name {
				continue
			}
			count++
			if count > 1 {
				return "", false
			}
			sessionField = field
		}
	}
	if count == 0 {
		return "", true
	}
	isolated := &http.Request{Header: http.Header{"Cookie": []string{sessionField}}}
	cookies := isolated.CookiesNamed(name)
	if len(cookies) != 1 {
		return "", true
	}
	return cookies[0].Value, true
}

func (h *AuthHandler) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := h.sessionToken(r)
		if !ok || token == "" {
			handleAuthError(w, app.ErrUnauthenticated)
			return
		}
		id, err := h.service.Authenticate(r.Context(), token, h.now())
		if err != nil {
			handleAuthError(w, err)
			return
		}
		if id == uuid.Nil {
			handleAuthError(w, app.ErrUnauthenticated)
			return
		}
		next.ServeHTTP(w, r.WithContext(app.WithUserID(r.Context(), id)))
	})
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrSessionUnavailable):
		response.WriteError(w, http.StatusServiceUnavailable, "authentication temporarily unavailable")
	case errors.Is(err, app.ErrInvalidCredentials):
		response.WriteError(w, http.StatusUnauthorized, "invalid login or password")
	case errors.Is(err, app.ErrUnauthenticated):
		response.WriteError(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, app.ErrValidation):
		response.WriteError(w, http.StatusBadRequest, "invalid request")
	case errors.Is(err, app.ErrLoginTaken):
		response.WriteError(w, http.StatusConflict, "login is already taken")
	default:
		response.WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}
