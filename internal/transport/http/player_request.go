package http

import (
	"bytes"
	"encoding/json"
	"unicode/utf8"

	"holdem-tournament-builder/internal/app"
)

func (req *PlayerRequest) UnmarshalJSON(data []byte) error {
	type playerBody PlayerRequest
	return decodePlayerJSON(data, (*playerBody)(req))
}

func decodePlayerJSON(data []byte, target any) error {
	if !utf8.Valid(data) {
		return app.ErrValidation
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
