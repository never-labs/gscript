-- Data-oriented hot benchmark reference: LuaJIT masked column aggregate.

local N = 65536
local REPS = 1200

local function make_value(n)
    local t = {}
    for i = 1, n do
        t[i] = 1.0 + (i % 257) * 0.125
    end
    return t
end

local function make_weight(n)
    local t = {}
    for i = 1, n do
        t[i] = 0.5 + (i % 31) * 0.01
    end
    return t
end

local function make_mask(n)
    local t = {}
    for i = 1, n do
        t[i] = i % 3 ~= 0
    end
    return t
end

local cols = {
    value = make_value(N),
    weight = make_weight(N),
    active = make_mask(N),
}

local function stats_where(values, mask)
    local count = 0
    local sum = 0.0
    local minv = nil
    local maxv = nil
    for i = 1, #values do
        if mask[i] then
            local v = values[i]
            count = count + 1
            sum = sum + v
            if minv == nil or v < minv then minv = v end
            if maxv == nil or v > maxv then maxv = v end
        end
    end
    return count, sum, minv, maxv, sum / count
end

local function run_hot(c, mask, reps)
    local checksum = 0.0
    for r = 1, reps do
        local count, sum, minv, maxv, mean = stats_where(c.value, mask)
        checksum = checksum + sum + mean + minv + maxv + count
    end
    return checksum
end

local warm = run_hot(cols, cols.active, 3)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, cols.active, REPS)
local elapsed = os.clock() - t0

print(string.format("soa_masked_aggregate_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
