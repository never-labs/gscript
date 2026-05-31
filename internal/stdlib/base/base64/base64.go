package base64

import stdbase64 "encoding/base64"

func Encode(s string) string {
	return stdbase64.StdEncoding.EncodeToString([]byte(s))
}

func Decode(s string) (string, error) {
	decoded, err := stdbase64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func URLEncode(s string) string {
	return stdbase64.RawURLEncoding.EncodeToString([]byte(s))
}

func URLDecode(s string) (string, error) {
	decoded, err := stdbase64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func EncodedLen(n int) int {
	return stdbase64.StdEncoding.EncodedLen(n)
}

func DecodedLen(n int) int {
	return stdbase64.StdEncoding.DecodedLen(n)
}

func URLEncodedLen(n int) int {
	return stdbase64.RawURLEncoding.EncodedLen(n)
}

func URLDecodedLen(n int) int {
	return stdbase64.RawURLEncoding.DecodedLen(n)
}
