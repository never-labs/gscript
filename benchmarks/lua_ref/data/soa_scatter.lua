-- Data-oriented hot benchmark reference: LuaJIT mask indexes plus scatter writes.

local N = 32768
local HALF = 16384
local REPS = 900

local function make_f64(n, scale, bias)
    local t = {}
    for i = 1, n do
        t[i] = i * scale + bias
    end
    return t
end

local function make_i64(n)
    local t = {}
    for i = 1, n do
        t[i] = i
    end
    return t
end

local function make_mask(n)
    local t = {}
    for i = 1, n do
        t[i] = i % 2 == 0
    end
    return t
end

local function make_values(n)
    local t = {}
    for i = 1, n do
        t[i] = i * 0.25
    end
    return t
end

local cols = {
    id = make_i64(N),
    score = make_f64(N, 0.01, 1.0),
    value = make_f64(N, 0.125, 0.5),
}
local mask = make_mask(N)
local values = make_values(HALF)

local function indices_where(m)
    local out = {}
    local j = 1
    for i = 1, #m do
        if m[i] then
            out[j] = i
            j = j + 1
        end
    end
    return out
end

local function scatter_into(dst, idx, values_or_scalar)
    if type(values_or_scalar) == "table" then
        for i = 1, #idx do
            dst[idx[i]] = values_or_scalar[i]
        end
    else
        for i = 1, #idx do
            dst[idx[i]] = values_or_scalar
        end
    end
end

local function sum_where(xs, m)
    local s = 0.0
    for i = 1, #m do
        if m[i] then
            s = s + xs[i]
        end
    end
    return s
end

local function run_hot(c, m, values, reps)
    local checksum = 0.0
    for r = 1, reps do
        local idx = indices_where(m)
        scatter_into(c.score, idx, values)
        checksum = checksum + sum_where(c.score, m)
        scatter_into(c.score, idx, 1.5)
        checksum = checksum + sum_where(c.score, m)
    end
    return checksum
end

local warm = run_hot(cols, mask, values, 2)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, mask, values, REPS)
local elapsed = os.clock() - t0

print(string.format("soa_scatter_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
