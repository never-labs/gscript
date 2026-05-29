-- Data-oriented hot benchmark reference: LuaJIT full-column fused stats reducer.

local N = 65536
local REPS = 2200

local function make_f64(n, scale, bias)
    local t = {}
    for i = 1, n do
        t[i] = i * scale + bias
    end
    return t
end

local cols = {
    value = make_f64(N, 0.01, -200.0),
}

local function stats_value(xs)
    local min_value = xs[1]
    local max_value = xs[1]
    local sum = xs[1]
    for i = 2, #xs do
        local v = xs[i]
        sum = sum + v
        if v < min_value then min_value = v end
        if v > max_value then max_value = v end
    end
    return {count = #xs, sum = sum, min = min_value, max = max_value, mean = sum / #xs}
end

local function run_hot(c, reps)
    local checksum = 0.0
    for r = 1, reps do
        local stats = stats_value(c.value)
        checksum = checksum + stats.count * 0.000001 + stats.sum * 0.000000001 + stats.min + stats.max + stats.mean
    end
    return checksum
end

local warm = run_hot(cols, 2)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, REPS)
local elapsed = os.clock() - t0

print(string.format("soa_reducers_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
