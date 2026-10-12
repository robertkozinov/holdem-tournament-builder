package http

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewServerAddressAndTimeouts(t *testing.T) {
	handler := http.NewServeMux()
	server := NewServer(handler, ":12345")
	assert.Equal(t, ":12345", server.server.Addr)
	assert.Same(t, handler, server.server.Handler)
	assert.Equal(t, 5*time.Second, server.server.ReadHeaderTimeout)
	assert.Equal(t, 10*time.Second, server.server.ReadTimeout)
	assert.Equal(t, 10*time.Second, server.server.WriteTimeout)
	assert.Equal(t, 60*time.Second, server.server.IdleTimeout)
}
