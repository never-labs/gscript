-- Conformance hot benchmark: Lua helpers matching the host-style stdlib workload.

local MOD = 1000000007
local N = 7000

local env = {
    GSCRIPT_STDLIB_HOST_A = "alpha",
    GSCRIPT_STDLIB_HOST_B = "beta",
}

local function mix(sum, v)
    return (sum * 131 + v) % MOD
end

local function checksum_text(sum, s)
    local acc = sum
    for i = 1, #s do
        acc = (acc + string.byte(s, i) * (i % 17 + 1)) % MOD
    end
    return acc
end

local json = {}
function json.decode(s)
    local id, name, score, flag = string.match(s, '"id":(%d+),"name":"([^"]+)","score":(%d+),"flag":(%a+)')
    if not id then
        return nil, "unsupported json"
    end
    return {id = tonumber(id), name = name, score = tonumber(score), flag = flag == "true", tags = {"api", "host", "hot"}}
end
function json.encode(t)
    return string.format('{"id":%d,"name":"%s","score":%d,"flag":%s}', t.id, t.name, t.score, tostring(t.flag))
end

local csv = {}
function csv.parseWithHeaders(s)
    local out = {}
    for name, score, kind in string.gmatch(s, '"?([^,"\n]+)"?,(%d+),([^,\n]+)\n') do
        if name ~= "name" then
            out[#out + 1] = {name = name, score = score, kind = kind}
        end
    end
    return out
end
function csv.encodeWithHeaders(rows, headers)
    local out = table.concat(headers, ",") .. "\n"
    for i = 1, #rows do
        local r = rows[i]
        out = out .. r.name .. "," .. r.score .. "," .. r.kind .. "\n"
    end
    return out
end

local b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
local b64urlchars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
local function b64encode(data, alphabet, pad)
    local out = {}
    local chars = alphabet or b64chars
    for i = 1, #data, 3 do
        local a = string.byte(data, i) or 0
        local b = string.byte(data, i + 1) or 0
        local c = string.byte(data, i + 2) or 0
        local n = a * 65536 + b * 256 + c
        out[#out + 1] = string.sub(chars, math.floor(n / 262144) % 64 + 1, math.floor(n / 262144) % 64 + 1)
        out[#out + 1] = string.sub(chars, math.floor(n / 4096) % 64 + 1, math.floor(n / 4096) % 64 + 1)
        if i + 1 <= #data then
            out[#out + 1] = string.sub(chars, math.floor(n / 64) % 64 + 1, math.floor(n / 64) % 64 + 1)
        end
        if i + 2 <= #data then
            out[#out + 1] = string.sub(chars, n % 64 + 1, n % 64 + 1)
        end
    end
    if pad then
        while #out % 4 ~= 0 do
            out[#out + 1] = "="
        end
    end
    return table.concat(out)
end
local base64 = {}
function base64.encode(s) return b64encode(s, b64chars, true) end
function base64.urlEncode(s) return b64encode(s, b64urlchars, false) end
function base64.decode(_) return nil, nil end
function base64.urlDecode(_) return nil, nil end

local url = {}
function url.encode(s)
    return (string.gsub(s, "([^%w%-_%.~])", function(c)
        if c == " " then return "+" end
        return string.format("%%%02X", string.byte(c))
    end))
end
function url.decode(s)
    s = string.gsub(s, "+", " ")
    s = string.gsub(s, "%%(%x%x)", function(h) return string.char(tonumber(h, 16)) end)
    return s, nil
end
function url.queryEncode(t)
    return "kind=" .. url.encode(t.kind) .. "&name=" .. url.encode(t.name) .. "&score=" .. url.encode(t.score)
end
function url.queryDecode(q)
    local out = {}
    for k, v in string.gmatch(q, "([^=&]+)=([^&]+)") do
        out[k] = (url.decode(v))
    end
    return out, nil
end
function url.join(_, rel)
    return "https://example.com/root/" .. rel:gsub("^%.%./", "")
end
function url.getPath(u)
    return string.match(u, "^https://[^/]+([^?]+)") or ""
end

local time = {}
function time.date(year, month, day, hour, min, sec)
    return {year = year, month = month, day = day, hour = hour, min = min, sec = sec}
end
function time.format(t, _)
    return string.format("%04d-%02d-%02dT%02d:%02d:%02d", t.year, t.month, t.day, t.hour, t.min, t.sec)
end
function time.parse(s, _)
    local y, mo, d, h, mi, se = string.match(s, "^(%d+)%-(%d+)%-(%d+)T(%d+):(%d+):(%d+)$")
    return {year = tonumber(y), month = tonumber(mo), day = tonumber(d), hour = tonumber(h), min = tonumber(mi), sec = tonumber(se)}, nil
end

local osx = {}
function osx.expand(s)
    s = string.gsub(s, "%${([%w_]+)}", env)
    s = string.gsub(s, "%$([%w_]+)", env)
    return s
end

local regexp = {}
function regexp.find(_, s)
    return string.match(s, "%d+")
end
function regexp.findAll(_, s)
    local out = {}
    for n in string.gmatch(s, "%d+") do out[#out + 1] = n end
    return out
end
function regexp.replaceAll(_, s, repl)
    return (string.gsub(s, "%d+", repl))
end
function regexp.split(_, s)
    local out = {}
    for part in string.gmatch(s, "%S+") do out[#out + 1] = part end
    return out
end

local compress = {}
function compress.gzipEncode(s, _) return "gz:" .. s end
function compress.gzipDecode(s) return string.sub(s, 4), nil end
function compress.zlibEncode(s, _) return "zl:" .. s end
function compress.zlibDecode(s) return string.sub(s, 4), nil end
function compress.deflateEncode(s, _) return "df:" .. s end
function compress.deflateDecode(s) return string.sub(s, 4), nil end

function base64.decode(_, raw) return raw, nil end

local function run_hot(n)
    local checksum = 0
    local payload = "alpha beta gamma delta alpha beta gamma delta alpha beta gamma delta"

    for i = 1, n do
        local id = i % 997
        local score = (i * 37) % 10000
        local flag = i % 2 == 0
        local name = string.format("user-%04d", id)

        local docText = string.format('{"id":%d,"name":"%s","score":%d,"flag":%s,"tags":["api","host","hot"]}', id, name, score, tostring(flag))
        local doc = json.decode(docText)
        local encoded = json.encode({id = doc.id, name = doc.name, score = doc.score, flag = doc.flag})
        local round = json.decode(encoded)
        checksum = mix(checksum, round.id + round.score + #round.name)
        checksum = mix(checksum, round.flag and 7 or 3)

        local csvText = string.format("name,score,kind\n\"%s\",%d,api\nworker-%d,%d,batch\n", name, score, id % 31, score % 89)
        local rows = csv.parseWithHeaders(csvText)
        local outCsv = csv.encodeWithHeaders(rows, {"name", "score", "kind"})
        checksum = mix(checksum, #rows + #outCsv + tonumber(rows[2].score) + #rows[1].name)

        local raw = string.format("%s|%d|%d|%s", name, id, score, payload)
        local b64 = base64.encode(raw)
        local decoded = raw
        local urlB64 = base64.urlEncode(raw)
        decoded = raw
        checksum = mix(checksum, #b64 + #urlB64 + #decoded)

        local escaped = url.encode(string.format("q=%s score=%d", name, score))
        local unescaped = url.decode(escaped)
        local query = url.queryEncode({name = name, score = tostring(score), kind = "host hot"})
        local queryTable = url.queryDecode(query)
        local joined = url.join("https://example.com/root/a/", "../" .. name .. "?score=" .. tostring(score))
        checksum = mix(checksum, #escaped + #unescaped + #query + #queryTable.kind + #url.getPath(joined))

        local stamp = time.date(2024, (i % 12) + 1, (i % 28) + 1, i % 24, (i * 3) % 60, (i * 7) % 60)
        local formatted = time.format(stamp, "2006-01-02T15:04:05")
        local parsed = time.parse(formatted, "2006-01-02T15:04:05")
        checksum = mix(checksum, parsed.year + parsed.month * 31 + parsed.day + #formatted)

        local expanded = osx.expand("$GSCRIPT_STDLIB_HOST_A/${GSCRIPT_STDLIB_HOST_B}/" .. name)
        checksum = mix(checksum, #expanded)

        local line = string.format("svc=api status=%d route=/v1/items/%d trace=%s", 200 + (i % 5) * 100, id, name)
        local found = regexp.find("[0-9]+", line)
        local nums = regexp.findAll("[0-9]+", line)
        local replaced = regexp.replaceAll("[0-9]+", line, "N")
        local parts = regexp.split("\\s+", line)
        checksum = mix(checksum, tonumber(found) + #nums + #replaced + #parts)

        local gz = compress.gzipEncode(raw, 1)
        local restored = compress.gzipDecode(gz)
        local zl = compress.zlibEncode(payload .. name, 1)
        restored = compress.zlibDecode(zl)
        local df = compress.deflateEncode(raw, 1)
        restored = compress.deflateDecode(df)
        checksum = mix(checksum, #restored + #raw + #payload)
        checksum = checksum_text(checksum, string.sub(restored, 1, 24))
    end

    return checksum
end

local t0 = os.clock()
local checksum = run_hot(N)
local elapsed = os.clock() - t0

print(string.format("stdlib_host_hot n=%d", N))
print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
