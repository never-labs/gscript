package dialect

import "testing"

func TestParseJWTUnverified(t *testing.T) {
	token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJ1c2VyLTQyIiwic2NvcGUiOiJyZWFkIHdyaXRlIiwiZXhwIjoxODkzNDU2MDAwfQ.signature"
	parts, err := ParseJWTUnverified(token)
	if err != nil {
		t.Fatalf("ParseJWTUnverified error: %v", err)
	}
	if parts.Header != `{"alg":"none","typ":"JWT"}` {
		t.Fatalf("header = %q", parts.Header)
	}
	if parts.Payload != `{"sub":"user-42","scope":"read write","exp":1893456000}` {
		t.Fatalf("payload = %q", parts.Payload)
	}
	if parts.Signature != "signature" || parts.SignatureSegment != "signature" {
		t.Fatalf("signature = %q segment %q", parts.Signature, parts.SignatureSegment)
	}
}

func TestParseJWTUnverifiedRejectsMalformedToken(t *testing.T) {
	for _, token := range []string{"", "one.two", ".payload.sig", "bad*.payload.sig"} {
		if _, err := ParseJWTUnverified(token); err == nil {
			t.Fatalf("ParseJWTUnverified(%q) returned nil error", token)
		}
	}
}
