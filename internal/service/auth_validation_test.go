package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"
)

func TestAuthCredentialBoundaries(t *testing.T) {
	for _, tc := range []struct {
		login, password, wantLogin string
	}{
		{"AbC", "12345678", "abc"},
		{strings.Repeat("A", 32), strings.Repeat("!", 128), strings.Repeat("a", 32)},
		{"._-", "ABCDEFGH", "._-"},
		{"  Player.1_-  ", "aA09!~{}", "player.1_-"},
	} {
		f := newAuthFixture()
		user, err := f.service.Register(context.Background(), tc.login, tc.password, authTestNow)
		require.NoError(t, err)
		require.Equal(t, tc.wantLogin, user.Login)
		require.Equal(t, tc.password, f.hasher.password)
	}
}

func TestAuthInvalidCredentialsFailBeforeDependencies(t *testing.T) {
	cases := []struct {
		name, login, password string
		want                  error
	}{
		{"empty login", "", "Password!9", app.ErrInvalidLogin},
		{"short login", "ab", "Password!9", app.ErrInvalidLogin},
		{"long login", strings.Repeat("a", 33), "Password!9", app.ErrInvalidLogin},
		{"space login", "ab cd", "Password!9", app.ErrInvalidLogin},
		{"only spaces", "   ", "Password!9", app.ErrInvalidLogin},
		{"unicode login", "игрок", "Password!9", app.ErrInvalidLogin},
		{"unicode casefold", "\u212Aelvin", "Password!9", app.ErrInvalidLogin},
		{"unicode space", "\u00a0abc", "Password!9", app.ErrInvalidLogin},
		{"tab login", "\tabc", "Password!9", app.ErrInvalidLogin},
		{"newline login", "abc\n", "Password!9", app.ErrInvalidLogin},
		{"control login", "ab\x00c", "Password!9", app.ErrInvalidLogin},
		{"symbol login", "abc@", "Password!9", app.ErrInvalidLogin},
		{"empty password", "abc", "", app.ErrInvalidPassword},
		{"short password", "abc", "1234567", app.ErrInvalidPassword},
		{"long password", "abc", strings.Repeat("a", 129), app.ErrInvalidPassword},
		{"leading space password", "abc", " Password!9", app.ErrInvalidPassword},
		{"trailing space password", "abc", "Password!9 ", app.ErrInvalidPassword},
		{"internal space password", "abc", "Pass word9", app.ErrInvalidPassword},
		{"unicode password", "abc", "Passwordé9", app.ErrInvalidPassword},
		{"control password", "abc", "Password\x009", app.ErrInvalidPassword},
		{"tab password", "abc", "Password\t9", app.ErrInvalidPassword},
		{"newline password", "abc", "Password9\n", app.ErrInvalidPassword},
		{"delete password", "abc", "Password\x7f9", app.ErrInvalidPassword},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, login := range []bool{false, true} {
				f := newAuthFixture()
				if login {
					result, err := f.service.Login(context.Background(), tc.login, tc.password, authTestNow)
					require.ErrorIs(t, err, app.ErrValidation)
					require.ErrorIs(t, err, tc.want)
					require.Equal(t, AuthResult{}, result)
				} else {
					user, err := f.service.Register(context.Background(), tc.login, tc.password, authTestNow)
					require.ErrorIs(t, err, app.ErrValidation)
					require.ErrorIs(t, err, tc.want)
					require.Equal(t, domain.User{}, user)
				}
				require.Zero(t, f.hasher.hashCalls)
				require.Zero(t, f.hasher.verifyCalls)
				require.Zero(t, f.users.createCalls)
				require.Zero(t, f.users.loginCalls)
				require.Zero(t, f.tokens.generateCalls)
				require.Zero(t, f.sessions.createCalls)
			}
		})
	}
}
