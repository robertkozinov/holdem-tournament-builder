package config

import (
	"fmt"
	"strconv"
	"time"
)

type RateLimit struct {
	RequestsPerMinute int
	Burst             int
}

type RequestSafety struct {
	General                   RateLimit
	Register                  RateLimit
	Login                     RateLimit
	MaxClients                int
	Idle                      time.Duration
	CleanupInterval           time.Duration
	MaxConcurrentAuthRequests int
}

func DefaultRequestSafety() RequestSafety {
	return RequestSafety{
		General:                   RateLimit{RequestsPerMinute: 120, Burst: 60},
		Register:                  RateLimit{RequestsPerMinute: 5, Burst: 3},
		Login:                     RateLimit{RequestsPerMinute: 10, Burst: 10},
		MaxClients:                10000,
		Idle:                      10 * time.Minute,
		CleanupInterval:           time.Minute,
		MaxConcurrentAuthRequests: 2,
	}
}

func loadRequestSafety(getenv func(string) string) (RequestSafety, error) {
	cfg := DefaultRequestSafety()
	for _, setting := range []struct {
		key    string
		target *int
	}{
		{"HTTP_RATE_LIMIT_REQUESTS_PER_MINUTE", &cfg.General.RequestsPerMinute},
		{"HTTP_RATE_LIMIT_BURST", &cfg.General.Burst},
		{"AUTH_REGISTER_REQUESTS_PER_MINUTE", &cfg.Register.RequestsPerMinute},
		{"AUTH_REGISTER_BURST", &cfg.Register.Burst},
		{"AUTH_LOGIN_REQUESTS_PER_MINUTE", &cfg.Login.RequestsPerMinute},
		{"AUTH_LOGIN_BURST", &cfg.Login.Burst},
		{"HTTP_RATE_LIMIT_MAX_CLIENTS", &cfg.MaxClients},
		{"AUTH_MAX_CONCURRENT_REQUESTS", &cfg.MaxConcurrentAuthRequests},
	} {
		value := getenv(setting.key)
		if value == "" {
			continue
		}
		number, err := strconv.Atoi(value)
		if err != nil || number <= 0 {
			return RequestSafety{}, fmt.Errorf("%s must be a positive integer", setting.key)
		}
		*setting.target = number
	}
	for _, setting := range []struct {
		key    string
		target *time.Duration
	}{
		{"HTTP_RATE_LIMIT_IDLE", &cfg.Idle},
		{"HTTP_RATE_LIMIT_CLEANUP_INTERVAL", &cfg.CleanupInterval},
	} {
		value := getenv(setting.key)
		if value == "" {
			continue
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return RequestSafety{}, fmt.Errorf("%s must be a positive duration", setting.key)
		}
		*setting.target = duration
	}
	return cfg, nil
}
