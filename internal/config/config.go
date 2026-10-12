package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	DatabaseURL     string
	RedisURL        string
	HTTPAddr        string
	CookieSecure    bool
	SessionLifetime time.Duration
	RequestSafety   RequestSafety
}

func Load(getenv func(string) string) (Config, error) {
	var cfg Config
	cfg.DatabaseURL = getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if _, err := pgxpool.ParseConfig(cfg.DatabaseURL); err != nil {
		return Config{}, fmt.Errorf("invalid DATABASE_URL")
	}
	cfg.RedisURL = getenv("REDIS_URL")
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}
	redisOptions, err := goredis.ParseURL(cfg.RedisURL)
	if err != nil {
		return Config{}, fmt.Errorf("invalid REDIS_URL")
	}
	if redisOptions.TLSConfig != nil && redisOptions.TLSConfig.InsecureSkipVerify {
		return Config{}, fmt.Errorf("REDIS_URL must verify TLS certificates")
	}
	parsed, err := url.Parse(cfg.RedisURL)
	if err != nil {
		return Config{}, fmt.Errorf("invalid REDIS_URL")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	local := host == "localhost" || (ip != nil && ip.IsLoopback())
	if parsed.Scheme != "rediss" && !local {
		return Config{}, fmt.Errorf("remote REDIS_URL must use TLS (rediss)")
	}
	cfg.CookieSecure = true
	if value := getenv("AUTH_COOKIE_SECURE"); value != "" {
		cfg.CookieSecure, err = strconv.ParseBool(value)
		if err != nil {
			return Config{}, fmt.Errorf("invalid AUTH_COOKIE_SECURE")
		}
	}
	cfg.SessionLifetime = 30 * 24 * time.Hour
	if value := getenv("AUTH_SESSION_LIFETIME"); value != "" {
		cfg.SessionLifetime, err = time.ParseDuration(value)
		if err != nil || cfg.SessionLifetime < time.Second || cfg.SessionLifetime > 30*24*time.Hour {
			return Config{}, fmt.Errorf("AUTH_SESSION_LIFETIME must be between 1s and 720h")
		}
	}
	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, fmt.Errorf("PORT must be between 1 and 65535")
	}
	cfg.HTTPAddr = ":" + strconv.Itoa(number)
	cfg.RequestSafety, err = loadRequestSafety(getenv)
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}
