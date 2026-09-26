package http

import (
	"context"
	"holdem-tournament-builder/internal/domain"
	"holdem-tournament-builder/internal/transport/response"
	"net/http"
	"time"
)

type currentUserKey struct{}

func CurrentUser(ctx context.Context) *domain.User {
	user, _ := ctx.Value(currentUserKey{}).(*domain.User)
	return user
}

func (h *AuthHandler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			response.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		user, err := h.service.Authenticate(r.Context(), cookie.Value, time.Now())
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if user == nil {
			response.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), currentUserKey{}, user)))
	})
}
