package dialect

import (
	"encoding/base64"
	"fmt"
	"strings"
)

type JWTParts struct {
	Header           string
	Payload          string
	Signature        string
	HeaderSegment    string
	PayloadSegment   string
	SignatureSegment string
}

func ParseJWTUnverified(token string) (JWTParts, error) {
	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return JWTParts{}, fmt.Errorf("jwt: token must have three segments")
	}
	if segments[0] == "" || segments[1] == "" {
		return JWTParts{}, fmt.Errorf("jwt: header and payload segments are required")
	}
	header, err := base64.RawURLEncoding.DecodeString(segments[0])
	if err != nil {
		return JWTParts{}, fmt.Errorf("jwt: invalid header segment: %w", err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		return JWTParts{}, fmt.Errorf("jwt: invalid payload segment: %w", err)
	}
	return JWTParts{
		Header:           string(header),
		Payload:          string(payload),
		Signature:        segments[2],
		HeaderSegment:    segments[0],
		PayloadSegment:   segments[1],
		SignatureSegment: segments[2],
	}, nil
}
