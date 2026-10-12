package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	valid := map[string]string{"DATABASE_URL": "postgres://user:placeholder@localhost/holdem", "REDIS_URL": "redis://127.0.0.1:6379/0"}
	t.Run("uses safe defaults", func(t *testing.T) {
		cfg, err := Load(func(key string) string { return valid[key] })
		require.NoError(t, err)
		assert.Equal(t, ":8080", cfg.HTTPAddr)
		assert.True(t, cfg.CookieSecure)
		assert.Equal(t, 30*24*time.Hour, cfg.SessionLifetime)
	})
	t.Run("allows explicit local cookies and port", func(t *testing.T) {
		env := map[string]string{"DATABASE_URL": valid["DATABASE_URL"], "REDIS_URL": valid["REDIS_URL"], "AUTH_COOKIE_SECURE": "false", "PORT": "8081", "AUTH_SESSION_LIFETIME": "24h"}
		cfg, err := Load(func(key string) string { return env[key] })
		require.NoError(t, err)
		assert.Equal(t, ":8081", cfg.HTTPAddr)
		assert.False(t, cfg.CookieSecure)
		assert.Equal(t, 24*time.Hour, cfg.SessionLifetime)
	})
	for _, tc := range []struct{ name, key, value string }{
		{"missing database", "DATABASE_URL", ""}, {"missing redis", "REDIS_URL", ""},
		{"invalid database", "DATABASE_URL", "postgres://user:secret@[invalid"},
		{"invalid redis", "REDIS_URL", "redis://user:secret@[invalid"},
		{"remote plaintext redis", "REDIS_URL", "redis://user:secret@remote.example:6379"},
		{"unverified TLS redis", "REDIS_URL", "rediss://user:secret@remote.example:6379?skip_verify=true"},
		{"bad cookie flag", "AUTH_COOKIE_SECURE", "sometimes"},
		{"zero lifetime", "AUTH_SESSION_LIFETIME", "0s"}, {"negative lifetime", "AUTH_SESSION_LIFETIME", "-1h"},
		{"too long lifetime", "AUTH_SESSION_LIFETIME", "721h"}, {"bad lifetime", "AUTH_SESSION_LIFETIME", "forever"},
		{"zero port", "PORT", "0"}, {"large port", "PORT", "65536"}, {"bad port", "PORT", "text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"DATABASE_URL": valid["DATABASE_URL"], "REDIS_URL": valid["REDIS_URL"], tc.key: tc.value}
			cfg, err := Load(func(key string) string { return env[key] })
			require.Error(t, err)
			assert.Equal(t, Config{}, cfg)
			assert.False(t, strings.Contains(err.Error(), "secret"))
		})
	}
	t.Run("accepts TLS Redis", func(t *testing.T) {
		env := map[string]string{"DATABASE_URL": valid["DATABASE_URL"], "REDIS_URL": "rediss://default:placeholder@sample.example:6379"}
		_, err := Load(func(key string) string { return env[key] })
		require.NoError(t, err)
	})
}
