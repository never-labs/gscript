package hash

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash/crc32"
)

// MD5 returns the lowercase hexadecimal MD5 digest of s.
func MD5(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// SHA1 returns the lowercase hexadecimal SHA-1 digest of s.
func SHA1(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// SHA256 returns the lowercase hexadecimal SHA-256 digest of s.
func SHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// SHA512 returns the lowercase hexadecimal SHA-512 digest of s.
func SHA512(s string) string {
	sum := sha512.Sum512([]byte(s))
	return hex.EncodeToString(sum[:])
}

// CRC32 returns the IEEE CRC-32 checksum of s.
func CRC32(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}

// HMACSHA256 returns the lowercase hexadecimal HMAC-SHA256 digest for message.
func HMACSHA256(key, message string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
