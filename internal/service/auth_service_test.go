package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/argon2"
	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/auth"
	"holdem-tournament-builder/internal/domain"
)

var authTestNow = time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
var authTestUserID = uuid.MustParse("00000000-0000-0000-0000-000000000042")

type authUserRepository struct {
	user                       domain.User
	createErr, loginErr, idErr error
	createCalls, loginCalls    int
	idCalls                    int
	requestedLogin             string
	requestedID                uuid.UUID
}

func (r *authUserRepository) Create(_ context.Context, user domain.User) error {
	r.createCalls++
	if r.createErr != nil {
		return r.createErr
	}
	r.user = user
	return nil
}

func (r *authUserRepository) GetByLogin(_ context.Context, login string) (domain.User, error) {
	r.loginCalls++
	r.requestedLogin = login
	return r.user, r.loginErr
}

func (r *authUserRepository) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	r.idCalls++
	r.requestedID = id
	return r.user, r.idErr
}

type authSessionStore struct {
	sessions                     map[string]domain.Session
	createErr, getErr, deleteErr error
	createCalls, getCalls        int
	deleteCalls                  int
	createdHash, requestedHash   string
}

func (s *authSessionStore) Create(_ context.Context, hash string, session domain.Session) error {
	s.createCalls++
	s.createdHash = hash
	if s.createErr != nil {
		return s.createErr
	}
	s.sessions[hash] = session
	return nil
}

func (s *authSessionStore) Get(_ context.Context, hash string) (domain.Session, error) {
	s.getCalls++
	s.requestedHash = hash
	session, ok := s.sessions[hash]
	if s.getErr != nil {
		return session, s.getErr
	}
	if !ok {
		return domain.Session{}, app.ErrSessionNotFound
	}
	return session, nil
}

func (s *authSessionStore) Delete(_ context.Context, hash string) error {
	s.deleteCalls++
	s.requestedHash = hash
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.sessions, hash)
	return nil
}

type authPasswordHasher struct {
	hashErr, verifyErr     error
	hashCalls, verifyCalls int
	password, encoded      string
}

func (h *authPasswordHasher) Hash(password string) (string, error) {
	h.hashCalls++
	h.password = password
	return "hashed:" + password, h.hashErr
}

func (h *authPasswordHasher) Verify(password, encoded string) (bool, error) {
	h.verifyCalls++
	h.password, h.encoded = password, encoded
	return encoded == "hashed:"+password, h.verifyErr
}

type authTokenGenerator struct {
	generateErr, hashErr     error
	generateCalls, hashCalls int
}

func (g *authTokenGenerator) Generate() (string, string, error) {
	g.generateCalls++
	if g.generateErr != nil {
		return "partial", "partial", g.generateErr
	}
	return (auth.SessionTokenGenerator{}).Generate()
}

func (g *authTokenGenerator) Hash(raw string) (string, error) {
	g.hashCalls++
	if g.hashErr != nil {
		return "partial", g.hashErr
	}
	return (auth.SessionTokenGenerator{}).Hash(raw)
}

type authFixture struct {
	users    *authUserRepository
	sessions *authSessionStore
	hasher   *authPasswordHasher
	tokens   *authTokenGenerator
	service  *AuthService
}

func newAuthFixture() authFixture {
	f := authFixture{
		users: &authUserRepository{user: domain.User{
			ID: authTestUserID, Login: "organizer", PasswordHash: "hashed:Password!9", CreatedAt: authTestNow.Add(-time.Hour),
		}},
		sessions: &authSessionStore{sessions: make(map[string]domain.Session)},
		hasher:   &authPasswordHasher{}, tokens: &authTokenGenerator{},
	}
	f.service = NewAuthService(f.users, f.sessions, f.hasher, f.tokens, 30*24*time.Hour)
	return f
}

func TestAuthRegisterNormalizesLoginAndPreservesPassword(t *testing.T) {
	f := newAuthFixture()
	user, err := f.service.Register(context.Background(), "  OrGaNiZeR_.-9  ", "Case-Sensitive!9", authTestNow)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, user.ID)
	require.Equal(t, "organizer_.-9", user.Login)
	require.Equal(t, authTestNow, user.CreatedAt)
	require.Equal(t, "hashed:Case-Sensitive!9", user.PasswordHash)
	require.Equal(t, "Case-Sensitive!9", f.hasher.password)
	require.Equal(t, user, f.users.user)
	require.Equal(t, 1, f.users.createCalls)
	require.Zero(t, f.users.loginCalls)
	require.Zero(t, f.tokens.generateCalls)
	require.Zero(t, f.sessions.createCalls)
}

func TestAuthRegisterOperationalErrors(t *testing.T) {
	cause := errors.New("operation failed")
	for _, tc := range []struct {
		name  string
		setup func(authFixture)
		want  error
	}{
		{"hash", func(f authFixture) { f.hasher.hashErr = cause }, cause},
		{"duplicate", func(f authFixture) { f.users.createErr = fmt.Errorf("duplicate: %w", app.ErrLoginTaken) }, app.ErrLoginTaken},
		{"create", func(f authFixture) { f.users.createErr = cause }, cause},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAuthFixture()
			tc.setup(f)
			user, err := f.service.Register(context.Background(), "Organizer", "Password!9", authTestNow)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, domain.User{}, user)
			require.Zero(t, f.tokens.generateCalls)
			require.Zero(t, f.sessions.createCalls)
			if tc.name == "hash" {
				require.Zero(t, f.users.createCalls)
			}
		})
	}
}

func TestAuthLoginCreatesFixedLifetimeSession(t *testing.T) {
	f := newAuthFixture()
	result, err := f.service.Login(context.Background(), "  OrGaNiZeR  ", "Password!9", authTestNow)
	require.NoError(t, err)
	require.Equal(t, authTestUserID, result.UserID)
	require.Equal(t, "organizer", result.Login)
	require.Equal(t, authTestNow.Add(30*24*time.Hour), result.ExpiresAt)
	require.Equal(t, "organizer", f.users.requestedLogin)
	require.Equal(t, "Password!9", f.hasher.password)
	require.Equal(t, "hashed:Password!9", f.hasher.encoded)
	hash, err := (auth.SessionTokenGenerator{}).Hash(result.Token)
	require.NoError(t, err)
	require.Equal(t, hash, f.sessions.createdHash)
	require.NotEqual(t, result.Token, f.sessions.createdHash)
	require.Equal(t, domain.Session{UserID: authTestUserID, CreatedAt: authTestNow, ExpiresAt: result.ExpiresAt}, f.sessions.sessions[hash])
	require.Equal(t, 1, f.tokens.generateCalls)
	require.Equal(t, 1, f.sessions.createCalls)
	second, err := f.service.Login(context.Background(), "organizer", "Password!9", authTestNow.Add(time.Hour))
	require.NoError(t, err)
	require.NotEqual(t, result.Token, second.Token)
	require.Len(t, f.sessions.sessions, 2)
}

func TestAuthLoginInvalidCredentialsAndDummyVerification(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			f := newAuthFixture()
			if missing {
				f.users.loginErr = fmt.Errorf("lookup: %w", app.ErrUserNotFound)
			}
			result, err := f.service.Login(context.Background(), "Organizer", "WrongPassword!9", authTestNow)
			require.ErrorIs(t, err, app.ErrInvalidCredentials)
			require.Equal(t, AuthResult{}, result)
			require.Equal(t, 1, f.hasher.verifyCalls)
			require.Equal(t, "WrongPassword!9", f.hasher.password)
			require.Zero(t, f.hasher.hashCalls)
			require.Zero(t, f.tokens.generateCalls)
			require.Zero(t, f.sessions.createCalls)
			if missing {
				_, err := (auth.PasswordHasher{}).Verify("WrongPassword!9", f.hasher.encoded)
				require.NoError(t, err)
				firstDummy := f.hasher.encoded
				_, err = f.service.Login(context.Background(), "another", "OtherPassword!9", authTestNow)
				require.ErrorIs(t, err, app.ErrInvalidCredentials)
				require.Equal(t, firstDummy, f.hasher.encoded)
			}
		})
	}
}

func TestAuthLoginOperationalErrors(t *testing.T) {
	cause := errors.New("operation failed")
	for _, tc := range []struct {
		name        string
		setup       func(authFixture)
		unavailable bool
	}{
		{"lookup", func(f authFixture) { f.users.loginErr = cause }, false},
		{"verify", func(f authFixture) { f.hasher.verifyErr = cause }, false},
		{"dummy verify", func(f authFixture) { f.users.loginErr = app.ErrUserNotFound; f.hasher.verifyErr = cause }, false},
		{"token", func(f authFixture) { f.tokens.generateErr = cause }, false},
		{"session", func(f authFixture) { f.sessions.createErr = cause }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAuthFixture()
			tc.setup(f)
			result, err := f.service.Login(context.Background(), "Organizer", "Password!9", authTestNow)
			require.ErrorIs(t, err, cause)
			require.NotErrorIs(t, err, app.ErrInvalidCredentials)
			require.Equal(t, AuthResult{}, result)
			if tc.unavailable {
				require.ErrorIs(t, err, app.ErrSessionUnavailable)
			} else {
				require.Zero(t, f.sessions.createCalls)
			}
			if tc.name == "lookup" {
				require.Zero(t, f.hasher.verifyCalls)
			}
		})
	}
}

func TestAuthAuthenticateValidatesSession(t *testing.T) {
	valid := domain.Session{UserID: authTestUserID, CreatedAt: authTestNow.Add(-time.Hour), ExpiresAt: authTestNow.Add(time.Hour)}
	cases := []struct {
		name    string
		session domain.Session
		now     time.Time
		want    error
	}{
		{"valid", valid, authTestNow, nil},
		{"at creation", valid, valid.CreatedAt, nil},
		{"just before expiry", valid, valid.ExpiresAt.Add(-time.Nanosecond), nil},
		{"at expiry", valid, valid.ExpiresAt, app.ErrUnauthenticated},
		{"expired", valid, valid.ExpiresAt.Add(time.Nanosecond), app.ErrUnauthenticated},
		{"future creation", valid, valid.CreatedAt.Add(-time.Nanosecond), app.ErrUnauthenticated},
		{"zero now", valid, time.Time{}, app.ErrUnauthenticated},
		{"nil user", domain.Session{CreatedAt: valid.CreatedAt, ExpiresAt: valid.ExpiresAt}, authTestNow, app.ErrUnauthenticated},
		{"zero creation", domain.Session{UserID: authTestUserID, ExpiresAt: valid.ExpiresAt}, authTestNow, app.ErrUnauthenticated},
		{"zero expiry", domain.Session{UserID: authTestUserID, CreatedAt: valid.CreatedAt}, authTestNow, app.ErrUnauthenticated},
		{"equal timestamps", domain.Session{UserID: authTestUserID, CreatedAt: authTestNow, ExpiresAt: authTestNow}, authTestNow, app.ErrUnauthenticated},
		{"reversed timestamps", domain.Session{UserID: authTestUserID, CreatedAt: valid.ExpiresAt, ExpiresAt: valid.CreatedAt}, authTestNow, app.ErrUnauthenticated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAuthFixture()
			raw, hash, err := (auth.SessionTokenGenerator{}).Generate()
			require.NoError(t, err)
			f.sessions.sessions[hash] = tc.session
			id, err := f.service.Authenticate(context.Background(), raw, tc.now)
			if tc.want == nil {
				require.NoError(t, err)
				require.Equal(t, authTestUserID, id)
			} else {
				require.ErrorIs(t, err, tc.want)
				require.Equal(t, uuid.Nil, id)
			}
			require.Equal(t, hash, f.sessions.requestedHash)
			require.Equal(t, tc.session, f.sessions.sessions[hash])
			require.Zero(t, f.sessions.createCalls)
			require.Zero(t, f.sessions.deleteCalls)
		})
	}
}

func TestAuthAuthenticateMalformedTokensNeverAccessStore(t *testing.T) {
	for _, token := range []string{"", "bad", strings.Repeat("A", 43) + "=", strings.Repeat("A", 42) + "B"} {
		f := newAuthFixture()
		id, err := f.service.Authenticate(context.Background(), token, authTestNow)
		require.ErrorIs(t, err, app.ErrUnauthenticated)
		require.Equal(t, uuid.Nil, id)
		require.Zero(t, f.sessions.getCalls)
	}
}

func TestAuthAuthenticateErrors(t *testing.T) {
	cause := errors.New("store offline")
	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{"missing", fmt.Errorf("get: %w", app.ErrSessionNotFound), app.ErrUnauthenticated},
		{"operational", cause, app.ErrSessionUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAuthFixture()
			f.sessions.getErr = tc.err
			raw, hash, err := (auth.SessionTokenGenerator{}).Generate()
			require.NoError(t, err)
			f.sessions.sessions[hash] = domain.Session{UserID: authTestUserID, CreatedAt: authTestNow, ExpiresAt: authTestNow.Add(time.Hour)}
			id, err := f.service.Authenticate(context.Background(), raw, authTestNow)
			require.ErrorIs(t, err, tc.want)
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, uuid.Nil, id)
		})
	}
	f := newAuthFixture()
	f.tokens.hashErr = cause
	id, err := f.service.Authenticate(context.Background(), strings.Repeat("A", 43), authTestNow)
	require.ErrorIs(t, err, cause)
	require.ErrorIs(t, err, app.ErrUnauthenticated)
	require.Equal(t, uuid.Nil, id)
	require.Zero(t, f.sessions.getCalls)
}

func TestAuthLogoutRevokesOnlyCurrentSession(t *testing.T) {
	f := newAuthFixture()
	first, err := f.service.Login(context.Background(), "organizer", "Password!9", authTestNow)
	require.NoError(t, err)
	second, err := f.service.Login(context.Background(), "organizer", "Password!9", authTestNow)
	require.NoError(t, err)
	require.NoError(t, f.service.Logout(context.Background(), first.Token))
	id, err := f.service.Authenticate(context.Background(), first.Token, authTestNow)
	require.ErrorIs(t, err, app.ErrUnauthenticated)
	require.Equal(t, uuid.Nil, id)
	id, err = f.service.Authenticate(context.Background(), second.Token, authTestNow)
	require.NoError(t, err)
	require.Equal(t, authTestUserID, id)
	require.NoError(t, f.service.Logout(context.Background(), first.Token))
	require.Len(t, f.sessions.sessions, 1)
}

func TestAuthLogoutMalformedAndMissingTokens(t *testing.T) {
	for _, token := range []string{"", "bad", strings.Repeat("A", 42) + "B"} {
		f := newAuthFixture()
		require.NoError(t, f.service.Logout(context.Background(), token))
		require.Zero(t, f.sessions.deleteCalls)
		require.Zero(t, f.sessions.getCalls)
	}
	f := newAuthFixture()
	f.sessions.deleteErr = fmt.Errorf("delete: %w", app.ErrSessionNotFound)
	require.NoError(t, f.service.Logout(context.Background(), strings.Repeat("A", 43)))
	require.Equal(t, 1, f.sessions.deleteCalls)
	require.Zero(t, f.sessions.getCalls)
}

func TestAuthLogoutOperationalFailure(t *testing.T) {
	f := newAuthFixture()
	cause := errors.New("store offline")
	f.sessions.deleteErr = cause
	err := f.service.Logout(context.Background(), strings.Repeat("A", 43))
	require.ErrorIs(t, err, app.ErrSessionUnavailable)
	require.ErrorIs(t, err, cause)
}

func TestAuthCurrentUser(t *testing.T) {
	f := newAuthFixture()
	user, err := f.service.CurrentUser(context.Background(), authTestUserID)
	require.NoError(t, err)
	require.Equal(t, f.users.user, user)
	require.Equal(t, authTestUserID, f.users.requestedID)
	for _, tc := range []struct {
		name  string
		id    uuid.UUID
		setup func(authFixture)
		want  error
	}{
		{"nil id", uuid.Nil, func(authFixture) {}, app.ErrUnauthenticated},
		{"missing", authTestUserID, func(f authFixture) { f.users.idErr = app.ErrUserNotFound }, app.ErrUnauthenticated},
		{"nil returned user", authTestUserID, func(f authFixture) { f.users.user.ID = uuid.Nil }, app.ErrUnauthenticated},
		{"different returned user", authTestUserID, func(f authFixture) { f.users.user.ID = uuid.MustParse("00000000-0000-0000-0000-000000000099") }, app.ErrUnauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAuthFixture()
			tc.setup(f)
			user, err := f.service.CurrentUser(context.Background(), tc.id)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, domain.User{}, user)
			if tc.id == uuid.Nil {
				require.Zero(t, f.users.idCalls)
			}
		})
	}
	f = newAuthFixture()
	cause := errors.New("database unavailable")
	f.users.idErr = cause
	user, err = f.service.CurrentUser(context.Background(), authTestUserID)
	require.ErrorIs(t, err, cause)
	require.NotErrorIs(t, err, app.ErrUnauthenticated)
	require.Equal(t, domain.User{}, user)
}

func TestAuthDummyHashUsesPublicNonSecretFixture(t *testing.T) {
	f := newAuthFixture()
	f.users.loginErr = app.ErrUserNotFound
	result, err := f.service.Login(context.Background(), "unknown", "PublicDummyPassword!9", authTestNow)
	require.ErrorIs(t, err, app.ErrInvalidCredentials)
	require.Equal(t, AuthResult{}, result)
	key := argon2.IDKey([]byte("PublicDummyPassword!9"), []byte("0123456789abcdef"), 2, 19456, 1, 32)
	_, encodedKey, ok := strings.Cut(f.hasher.encoded, "$MDEyMzQ1Njc4OWFiY2RlZg$")
	require.True(t, ok)
	require.Equal(t, base64.RawStdEncoding.EncodeToString(key), encodedKey)
	verified, err := (auth.PasswordHasher{}).Verify("PublicDummyPassword!9", f.hasher.encoded)
	require.NoError(t, err)
	require.True(t, verified)
	require.Zero(t, f.sessions.createCalls)
	realService := NewAuthService(f.users, f.sessions, auth.PasswordHasher{}, f.tokens, 30*24*time.Hour)
	result, err = realService.Login(context.Background(), "unknown", "PublicDummyPassword!9", authTestNow)
	require.ErrorIs(t, err, app.ErrInvalidCredentials)
	require.Equal(t, AuthResult{}, result)
	require.Zero(t, f.tokens.generateCalls)
	require.Zero(t, f.sessions.createCalls)
}

type authFailingRandomReader struct{ err error }

func (r authFailingRandomReader) Read([]byte) (int, error) { return 0, r.err }

func TestAuthRegisterUserIDGenerationFailure(t *testing.T) {
	f := newAuthFixture()
	cause := errors.New("user id randomness unavailable")
	uuid.SetRand(authFailingRandomReader{cause})
	t.Cleanup(func() { uuid.SetRand(nil) })
	user, err := f.service.Register(context.Background(), "organizer", "Password!9", authTestNow)
	require.ErrorIs(t, err, cause)
	require.Equal(t, domain.User{}, user)
	require.Zero(t, f.users.createCalls)
	require.Zero(t, f.tokens.generateCalls)
	require.Zero(t, f.sessions.createCalls)
}

func TestAuthLifecycleWithRealCryptography(t *testing.T) {
	users := &authUserRepository{}
	sessions := &authSessionStore{sessions: make(map[string]domain.Session)}
	service := NewAuthService(users, sessions, auth.PasswordHasher{}, auth.SessionTokenGenerator{}, 2*time.Hour)
	ctx := context.Background()
	user, err := service.Register(ctx, "  Real.Player  ", "UnmodifiedPassword!9", authTestNow)
	require.NoError(t, err)
	require.Equal(t, "real.player", user.Login)
	require.NotEqual(t, "UnmodifiedPassword!9", user.PasswordHash)
	require.Empty(t, sessions.sessions)
	result, err := service.Login(ctx, "REAL.PLAYER", "UnmodifiedPassword!9", authTestNow)
	require.NoError(t, err)
	require.Equal(t, authTestNow.Add(2*time.Hour), result.ExpiresAt)
	id, err := service.Authenticate(ctx, result.Token, authTestNow.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, user.ID, id)
	current, err := service.CurrentUser(ctx, id)
	require.NoError(t, err)
	require.Equal(t, user, current)
	id, err = service.Authenticate(ctx, result.Token, result.ExpiresAt)
	require.ErrorIs(t, err, app.ErrUnauthenticated)
	require.Equal(t, uuid.Nil, id)
	wrong, err := service.Login(ctx, "real.player", "unmodifiedPassword!9", authTestNow)
	require.ErrorIs(t, err, app.ErrInvalidCredentials)
	require.Equal(t, AuthResult{}, wrong)
	require.NoError(t, service.Logout(ctx, result.Token))
	id, err = service.Authenticate(ctx, result.Token, authTestNow)
	require.ErrorIs(t, err, app.ErrUnauthenticated)
	require.Equal(t, uuid.Nil, id)
}

func TestAuthLoginRejectsNilUserID(t *testing.T) {
	f := newAuthFixture()
	f.users.user.ID = uuid.Nil
	result, err := f.service.Login(context.Background(), "organizer", "Password!9", authTestNow)
	require.ErrorIs(t, err, app.ErrInvalidCredentials)
	require.Equal(t, AuthResult{}, result)
	require.Zero(t, f.tokens.generateCalls)
	require.Zero(t, f.sessions.createCalls)
}
