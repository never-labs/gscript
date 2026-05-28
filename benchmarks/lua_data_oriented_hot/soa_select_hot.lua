-- Data-oriented hot benchmark reference: LuaJIT mask select into tables.

local N = 65536
local REPS = 1200

local function make_value(n)
    local t = {}
    for i = 1, n do
        t[i] = 1.0 + (i % 257) * 0.125
    end
    return t
end

local function make_fallback(n)
    local t = {}
    for i = 1, n do
        t[i] = 0.25 + (i % 17) * 0.5
    end
    return t
end

local cols = {
    value = make_value(N),
    fallback = make_fallback(N),
    selected = {},
}
for i = 1, N do
    cols.selected[i] = 0
end

local function make_mask(values)
    local out = {}
    for i = 1, #values do
        out[i] = values[i] >= 8.5
    end
    return out
end

local mask = make_mask(cols.value)

local function sum_select(mask, if_true, if_false)
    local sum = 0.0
    for i = 1, #mask do
        if mask[i] then
            sum = sum + if_true[i]
        else
            sum = sum + if_false[i]
        end
    end
    return sum
end

local function run_hot(c, m, reps)
    local checksum = 0.0
    for r = 1, reps do
        checksum = checksum + sum_select(m, c.value, c.fallback)
    end
    return checksum
end

local warm = run_hot(cols, mask, 3)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, mask, REPS)
local elapsed = os.clock() - t0

print(string.format("soa_select_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
