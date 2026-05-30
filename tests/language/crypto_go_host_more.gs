print("case:crypto_go_host_more")

assert(crypto.equal("secret", "secret"))
assert(!crypto.equal("secret", "public"))
assert(!crypto.equal("short", "longer"))
assert(crypto.equal("", ""))

key16 := "1234567890abcdef"
plaintext := "hello from gscript"
ciphertext := crypto.aesGcmEncrypt(key16, plaintext)
assert(type(ciphertext) == "string")
assert(#ciphertext > #plaintext)
assert(crypto.aesGcmDecrypt(key16, ciphertext) == plaintext)

emptyCipher := crypto.aesGcmEncrypt(key16, "")
assert(crypto.aesGcmDecrypt(key16, emptyCipher) == "")

badHex, badHexErr := crypto.aesGcmDecrypt(key16, "not-hex")
assert(badHex == nil && badHexErr == "invalid hex ciphertext")
shortCipher, shortCipherErr := crypto.aesGcmDecrypt(key16, "001122")
assert(shortCipher == nil && shortCipherErr == "ciphertext too short")
badKey, badKeyErr := crypto.aesGcmDecrypt("bad", ciphertext)
assert(badKey == nil && type(badKeyErr) == "string" && #badKeyErr > 0)

bytes0 := crypto.randomBytes(0)
assert(bytes0 == "")
bytes8 := crypto.randomBytes(8)
assert(type(bytes8) == "string" && #bytes8 == 8)
hex0 := crypto.randomHex(0)
assert(hex0 == "")
hex8 := crypto.randomHex(8)
assert(type(hex8) == "string" && #hex8 == 16)
assert(string.match(hex8, "^[0-9a-f]+$") == hex8)

keyDefault := crypto.generateKey()
key24 := crypto.generateKey(24)
assert(type(keyDefault) == "string" && #keyDefault == 32)
assert(type(key24) == "string" && #key24 == 24)
ok, err := pcall(func() { crypto.generateKey(15) })
assert(!ok && type(err) == "string")

print("ok")
