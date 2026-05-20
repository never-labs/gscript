// Official hot benchmark: host-style stdlib CPU paths without file/network IO.

MOD := 1000000007
N := 7000

os.setenv("GSCRIPT_STDLIB_HOST_A", "alpha")
os.setenv("GSCRIPT_STDLIB_HOST_B", "beta")

func mix(sum, v) {
    return (sum * 131 + v) % MOD
}

func checksum_text(sum, s) {
    local := sum
    for i := 1; i <= #s; i++ {
        local = (local + string.byte(s, i) * (i % 17 + 1)) % MOD
    }
    return local
}

func run_hot(n) {
    checksum := 0
    payload := "alpha beta gamma delta alpha beta gamma delta alpha beta gamma delta"

    for i := 1; i <= n; i++ {
        id := i % 997
        score := (i * 37) % 10000
        flag := i % 2 == 0
        name := string.format("user-%04d", id)

        docText := string.format("{\"id\":%d,\"name\":\"%s\",\"score\":%d,\"flag\":%s,\"tags\":[\"api\",\"host\",\"hot\"]}", id, name, score, tostring(flag))
        doc := json.decode(docText)
        encoded := json.encode({id: doc.id, name: doc.name, score: doc.score, flag: doc.flag})
        round := json.decode(encoded)
        checksum = mix(checksum, round.id + round.score + #round.name)
        if round.flag {
            checksum = mix(checksum, 7)
        } else {
            checksum = mix(checksum, 3)
        }

        csvText := string.format("name,score,kind\n\"%s\",%d,api\nworker-%d,%d,batch\n", name, score, id % 31, score % 89)
        rows := csv.parseWithHeaders(csvText)
        outCsv := csv.encodeWithHeaders(rows, {"name", "score", "kind"})
        checksum = mix(checksum, #rows + #outCsv + tonumber(rows[2].score) + #rows[1].name)

        raw := string.format("%s|%d|%d|%s", name, id, score, payload)
        b64 := base64.encode(raw)
        decoded, b64err := base64.decode(b64)
        assert(b64err == nil && decoded == raw)
        urlB64 := base64.urlEncode(raw)
        decoded, b64err = base64.urlDecode(urlB64)
        assert(b64err == nil && decoded == raw)
        checksum = mix(checksum, #b64 + #urlB64 + #decoded)

        escaped := url.encode(string.format("q=%s score=%d", name, score))
        unescaped, urlErr := url.decode(escaped)
        assert(urlErr == nil)
        query := url.queryEncode({name: name, score: tostring(score), kind: "host hot"})
        queryTable, queryErr := url.queryDecode(query)
        assert(queryErr == nil)
        joined := url.join("https://example.com/root/a/", "../" .. name .. "?score=" .. tostring(score))
        checksum = mix(checksum, #escaped + #unescaped + #query + #queryTable.kind + #url.getPath(joined))

        stamp := time.date(2024, (i % 12) + 1, (i % 28) + 1, i % 24, (i * 3) % 60, (i * 7) % 60)
        formatted := time.format(stamp, "2006-01-02T15:04:05")
        parsed, parseErr := time.parse(formatted, "2006-01-02T15:04:05")
        assert(parseErr == nil)
        checksum = mix(checksum, parsed.year + parsed.month * 31 + parsed.day + #formatted)

        expanded := os.expand("$GSCRIPT_STDLIB_HOST_A/${GSCRIPT_STDLIB_HOST_B}/" .. name)
        checksum = mix(checksum, #expanded)

        line := string.format("svc=api status=%d route=/v1/items/%d trace=%s", 200 + (i % 5) * 100, id, name)
        found := regexp.find("[0-9]+", line)
        nums := regexp.findAll("[0-9]+", line)
        replaced := regexp.replaceAll("[0-9]+", line, "N")
        parts := regexp.split("\\s+", line)
        checksum = mix(checksum, tonumber(found) + #nums + #replaced + #parts)

        gz := compress.gzipEncode(raw, 1)
        restored, compErr := compress.gzipDecode(gz)
        assert(compErr == nil && restored == raw)
        zl := compress.zlibEncode(payload .. name, 1)
        restored, compErr = compress.zlibDecode(zl)
        assert(compErr == nil && restored == payload .. name)
        df := compress.deflateEncode(raw, 1)
        restored, compErr = compress.deflateDecode(df)
        assert(compErr == nil && restored == raw)
        checksum = mix(checksum, #restored + #raw + #payload)
        checksum = checksum_text(checksum, string.sub(restored, 1, 24))
    }

    os.unsetenv("GSCRIPT_STDLIB_HOST_A")
    os.unsetenv("GSCRIPT_STDLIB_HOST_B")
    return checksum
}

t0 := time.now()
checksum := run_hot(N)
elapsed := time.since(t0)

print(string.format("stdlib_host_hot n=%d", N))
print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
