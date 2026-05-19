print("case:compress_error_levels_more")

payload := "abc abc abc abc"

gz := compress.gzipEncode(payload, 1)
out, err := compress.gzipDecode(gz)
assert(out == payload)
assert(err == nil)

gzDefault := compress.gzipEncode(payload, -1)
out, err = compress.gzipDecode(gzDefault)
assert(out == payload)
assert(err == nil)

zl := compress.zlibEncode(payload, 9)
out, err = compress.zlibDecode(zl)
assert(out == payload)
assert(err == nil)

zlDefault := compress.zlibEncode(payload, 99)
out, err = compress.zlibDecode(zlDefault)
assert(out == payload)
assert(err == nil)

df := compress.deflateEncode(payload, 1)
out, err = compress.deflateDecode(df)
assert(out == payload)
assert(err == nil)

dfDefault := compress.deflateEncode(payload, -5)
out, err = compress.deflateDecode(dfDefault)
assert(out == payload)
assert(err == nil)

out, err = compress.zlibDecode("not-zlib")
assert(out == nil)
assert(type(err) == "string")

out, err = compress.deflateDecode("not-deflate")
assert(out == nil)
assert(type(err) == "string")

assert(!pcall(compress.gzipEncode))
assert(!pcall(compress.zlibEncode))
assert(!pcall(compress.deflateEncode))
assert(!pcall(compress.gzipDecode))
assert(!pcall(compress.zlibDecode))
assert(!pcall(compress.deflateDecode))

print("ok")
