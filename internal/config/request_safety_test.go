package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRequestSafety(t *testing.T) {
	defaults := RequestSafety{
		General:                   RateLimit{RequestsPerMinute: 120, Burst: 60},
		Register:                  RateLimit{RequestsPerMinute: 5, Burst: 3},
		Login:                     RateLimit{RequestsPerMinute: 10, Burst: 10},
		MaxClients:                10000,
		Idle:                      10 * time.Minute,
		CleanupInterval:           time.Minute,
		MaxConcurrentAuthRequests: 2,
	}
	t.Run("uses existing defaults", func(t *testing.T) {
		cfg, err := loadRequestSafetyTestConfig(nil)
		require.NoError(t, err)
		assert.Equal(t, defaults, cfg.RequestSafety)
	})
	t.Run("loads all explicit settings", func(t *testing.T) {
		cfg, err := loadRequestSafetyTestConfig(map[string]string{
			"HTTP_RATE_LIMIT_REQUESTS_PER_MINUTE": "240",
			"HTTP_RATE_LIMIT_BURST":               "80",
			"AUTH_REGISTER_REQUESTS_PER_MINUTE":   "12",
			"AUTH_REGISTER_BURST":                 "4",
			"AUTH_LOGIN_REQUESTS_PER_MINUTE":      "30",
			"AUTH_LOGIN_BURST":                    "6",
			"HTTP_RATE_LIMIT_MAX_CLIENTS":         "2000",
			"HTTP_RATE_LIMIT_IDLE":                "5m",
			"HTTP_RATE_LIMIT_CLEANUP_INTERVAL":    "30s",
			"AUTH_MAX_CONCURRENT_REQUESTS":        "4",
		})
		require.NoError(t, err)
		assert.Equal(t, RequestSafety{
			General:                   RateLimit{RequestsPerMinute: 240, Burst: 80},
			Register:                  RateLimit{RequestsPerMinute: 12, Burst: 4},
			Login:                     RateLimit{RequestsPerMinute: 30, Burst: 6},
			MaxClients:                2000,
			Idle:                      5 * time.Minute,
			CleanupInterval:           30 * time.Second,
			MaxConcurrentAuthRequests: 4,
		}, cfg.RequestSafety)
	})
	t.Run("empty settings retain defaults", func(t *testing.T) {
		cfg, err := loadRequestSafetyTestConfig(map[string]string{
			"HTTP_RATE_LIMIT_REQUESTS_PER_MINUTE": "",
			"HTTP_RATE_LIMIT_BURST":               "",
			"AUTH_REGISTER_REQUESTS_PER_MINUTE":   "",
			"AUTH_REGISTER_BURST":                 "",
			"AUTH_LOGIN_REQUESTS_PER_MINUTE":      "",
			"AUTH_LOGIN_BURST":                    "",
			"HTTP_RATE_LIMIT_MAX_CLIENTS":         "",
			"HTTP_RATE_LIMIT_IDLE":                "",
			"HTTP_RATE_LIMIT_CLEANUP_INTERVAL":    "",
			"AUTH_MAX_CONCURRENT_REQUESTS":        "",
		})
		require.NoError(t, err)
		assert.Equal(t, defaults, cfg.RequestSafety)
	})
	t.Run("individual override preserves other defaults", func(t *testing.T) {
		cfg, err := loadRequestSafetyTestConfig(map[string]string{"AUTH_MAX_CONCURRENT_REQUESTS": "1"})
		require.NoError(t, err)
		want := defaults
		want.MaxConcurrentAuthRequests = 1
		assert.Equal(t, want, cfg.RequestSafety)
	})
	t.Run("accepts independent durations and small positive values", func(t *testing.T) {
		cfg, err := loadRequestSafetyTestConfig(map[string]string{
			"HTTP_RATE_LIMIT_REQUESTS_PER_MINUTE": "1",
			"HTTP_RATE_LIMIT_BURST":               "1",
			"AUTH_REGISTER_REQUESTS_PER_MINUTE":   "1",
			"AUTH_REGISTER_BURST":                 "1",
			"AUTH_LOGIN_REQUESTS_PER_MINUTE":      "1",
			"AUTH_LOGIN_BURST":                    "1",
			"HTTP_RATE_LIMIT_MAX_CLIENTS":         "1",
			"HTTP_RATE_LIMIT_IDLE":                "1ns",
			"HTTP_RATE_LIMIT_CLEANUP_INTERVAL":    "1m",
			"AUTH_MAX_CONCURRENT_REQUESTS":        "1",
		})
		require.NoError(t, err)
		assert.Equal(t, RequestSafety{
			General:                   RateLimit{RequestsPerMinute: 1, Burst: 1},
			Register:                  RateLimit{RequestsPerMinute: 1, Burst: 1},
			Login:                     RateLimit{RequestsPerMinute: 1, Burst: 1},
			MaxClients:                1,
			Idle:                      time.Nanosecond,
			CleanupInterval:           time.Minute,
			MaxConcurrentAuthRequests: 1,
		}, cfg.RequestSafety)
	})
}

func TestLoadRejectsInvalidRequestSafety(t *testing.T) {
	intKeys := []string{
		"HTTP_RATE_LIMIT_REQUESTS_PER_MINUTE",
		"HTTP_RATE_LIMIT_BURST",
		"AUTH_REGISTER_REQUESTS_PER_MINUTE",
		"AUTH_REGISTER_BURST",
		"AUTH_LOGIN_REQUESTS_PER_MINUTE",
		"AUTH_LOGIN_BURST",
		"HTTP_RATE_LIMIT_MAX_CLIENTS",
		"AUTH_MAX_CONCURRENT_REQUESTS",
	}
	durationKeys := []string{
		"HTTP_RATE_LIMIT_IDLE",
		"HTTP_RATE_LIMIT_CLEANUP_INTERVAL",
	}
	for _, key := range intKeys {
		for _, value := range []string{"0", "-1", "1.5", "secret-invalid-value", " ", "999999999999999999999999999999999999"} {
			t.Run(key+"/"+value, func(t *testing.T) {
				assertInvalidRequestSafety(t, key, value)
			})
		}
	}
	for _, key := range durationKeys {
		for _, value := range []string{"0s", "-1s", "secret-invalid-value", " ", "10", "999999999999999999999h"} {
			t.Run(key+"/"+value, func(t *testing.T) {
				assertInvalidRequestSafety(t, key, value)
			})
		}
	}
}

func assertInvalidRequestSafety(t *testing.T, key, value string) {
	t.Helper()
	cfg, err := loadRequestSafetyTestConfig(map[string]string{key: value})
	require.Error(t, err)
	assert.Equal(t, Config{}, cfg)
	assert.Contains(t, err.Error(), key)
	if value != " " {
		assert.NotContains(t, err.Error(), value)
	}
}

func loadRequestSafetyTestConfig(overrides map[string]string) (Config, error) {
	env := map[string]string{
		"DATABASE_URL": "postgres://user:placeholder@localhost/holdem",
		"REDIS_URL":    "redis://127.0.0.1:6379/0",
	}
	for key, value := range overrides {
		env[key] = value
	}
	return Load(func(key string) string { return env[key] })
}
