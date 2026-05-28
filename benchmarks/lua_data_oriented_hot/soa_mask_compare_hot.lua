-- Data-oriented hot benchmark reference: LuaJIT column comparisons that produce masks.

local N = 65536
local REPS = 1000

local function make_value(n)
    local t = {}
    for i = 1, n do
        t[i] = 1.0 + (i % 257) * 0.125
    end
    return t
end

local function make_limit(n)
    local t = {}
    for i = 1, n do
        t[i] = 12.0 + (i % 19) * 0.25
    end
    return t
end

local cols = {
    value = make_value(N),
    limit = make_limit(N),
}

local function scalar_mask(values, threshold)
    local out = {}
    for i = 1, #values do
        out[i] = values[i] >= threshold
    end
    return out
end

local function column_mask(values, limits)
    local out = {}
    for i = 1, #values do
        out[i] = values[i] < limits[i]
    end
    return out
end

local function count_where(mask)
    local count = 0
    for i = 1, #mask do
        if mask[i] then
            count = count + 1
        end
    end
    return count
end

local function sum_where(values, mask)
    local sum = 0.0
    for i = 1, #values do
        if mask[i] then
            sum = sum + values[i]
        end
    end
    return sum
end

local function run_hot(c, reps)
    local checksum = 0.0
    for r = 1, reps do
        local sm = scalar_mask(c.value, 8.5)
        local cm = column_mask(c.value, c.limit)
        checksum = checksum + count_where(sm)
        checksum = checksum + sum_where(c.value, cm)
    end
    return checksum
end

local warm = run_hot(cols, 3)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, REPS)
local elapsed = os.clock() - t0

print(string.format("soa_mask_compare_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
