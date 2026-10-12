package postgres

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseTestPostgresURL(raw string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(raw)
	if err != nil {
		return nil, errors.New("invalid TEST_DATABASE_URL")
	}
	if !strings.HasSuffix(cfg.ConnConfig.Database, "_test") {
		return nil, errors.New("TEST_DATABASE_URL must name a database ending in _test")
	}
	return cfg, nil
}

func parseTestRedisURL(raw string) (*goredis.Options, error) {
	opts, err := goredis.ParseURL(raw)
	if err != nil {
		return nil, errors.New("invalid TEST_REDIS_URL")
	}
	host, _, err := net.SplitHostPort(opts.Addr)
	ip := net.ParseIP(host)
	if err != nil || !(host == "localhost" || (ip != nil && ip.IsLoopback())) {
		return nil, errors.New("only isolated local Redis is allowed in integration tests")
	}
	return opts, nil
}

func TestTestURLParsingDoesNotExposeCredentials(t *testing.T) {
	for _, parse := range []struct {
		name string
		run  func() error
	}{
		{"postgres", func() error { _, err := parseTestPostgresURL("postgres://user:test-secret@[invalid"); return err }},
		{"redis", func() error { _, err := parseTestRedisURL("redis://user:test-secret@[invalid"); return err }},
	} {
		t.Run(parse.name, func(t *testing.T) {
			err := parse.run()
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "test-secret")
			assert.NotContains(t, err.Error(), "://")
		})
	}
}
