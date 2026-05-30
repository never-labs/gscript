-- Data-oriented hot benchmark reference: LuaJIT dense prefix scan and scanInto.

local N = 65536
local REPS = 1400

local function make_f64(n, scale, bias)
    local t = {}
    for i = 1, n do
        t[i] = i * scale + bias
    end
    return t
end

local cols = {
    value = make_f64(N, 0.001, 1.0),
    prefix = make_f64(N, 0.0, 0.0),
}

local function scan(src)
    local out = {}
    local sum = 0.0
    for i = 1, #src do
        sum = sum + src[i]
        out[i] = sum
    end
    return out
end

local function scan_into(dst, src)
    local sum = 0.0
    for i = 1, #src do
        sum = sum + src[i]
        dst[i] = sum
    end
end

local function run_hot(c, reps)
    local checksum = 0.0
    for r = 1, reps do
        local tmp = scan(c.value)
        checksum = checksum + tmp[N]
        scan_into(c.prefix, c.value)
        checksum = checksum + c.prefix[N]
    end
    return checksum
end

local warm = run_hot(cols, 2)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, REPS)
local elapsed = os.clock() - t0

print(string.format("soa_scan_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
