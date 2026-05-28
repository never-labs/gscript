-- Data-oriented hot benchmark reference: LuaJIT dense column clamp and clampInto.

local N = 65536
local REPS = 1600

local function make_f64(n, scale, bias)
    local t = {}
    for i = 1, n do
        t[i] = i * scale + bias
    end
    return t
end

local cols = {
    value = make_f64(N, 0.01, -200.0),
    clamped = make_f64(N, 0.0, 0.0),
}

local function clamp(src, min_value, max_value)
    local out = {}
    for i = 1, #src do
        local v = src[i]
        if v < min_value then
            v = min_value
        elseif v > max_value then
            v = max_value
        end
        out[i] = v
    end
    return out
end

local function clamp_into(dst, src, min_value, max_value)
    for i = 1, #src do
        local v = src[i]
        if v < min_value then
            v = min_value
        elseif v > max_value then
            v = max_value
        end
        dst[i] = v
    end
end

local function run_hot(c, reps)
    local checksum = 0.0
    for r = 1, reps do
        local tmp = clamp(c.value, -25.0, 125.0)
        checksum = checksum + tmp[1] + tmp[N]
        clamp_into(c.clamped, c.value, -25.0, 125.0)
        checksum = checksum + c.clamped[1] + c.clamped[N]
    end
    return checksum
end

local warm = run_hot(cols, 2)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, REPS)
local elapsed = os.clock() - t0

print(string.format("soa_clamp_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
