-- Data-oriented hot benchmark reference: LuaJIT column-array filter/gather.

local N = 32768
local FILTER_REPS = 180
local GATHER_REPS = 180

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
        t[i] = i % 4 ~= 0
    end
    return t
end

local function make_indices(n)
    local t = {}
    local out = 1
    for i = n, 1, -3 do
        t[out] = i
        out = out + 1
    end
    return t
end

local cols = {
    id = make_i64(N),
    x = make_f64(N, 0.001, 1.0),
    y = make_f64(N, 0.002, 2.0),
    value = make_f64(N, 0.125, 0.5),
    weight = make_f64(N, 0.01, 0.25),
}
local mask = make_mask(N)
local indices = make_indices(N)

local function sum(a)
    local s = 0.0
    for i = 1, #a do
        s = s + a[i]
    end
    return s
end

local function filter_cols(c, m)
    local out = {id = {}, x = {}, y = {}, value = {}, weight = {}}
    local j = 1
    for i = 1, #m do
        if m[i] then
            out.id[j] = c.id[i]
            out.x[j] = c.x[i]
            out.y[j] = c.y[i]
            out.value[j] = c.value[i]
            out.weight[j] = c.weight[i]
            j = j + 1
        end
    end
    return out
end

local function gather_cols(c, idx)
    local out = {id = {}, x = {}, y = {}, value = {}, weight = {}}
    for i = 1, #idx do
        local at = idx[i]
        out.id[i] = c.id[at]
        out.x[i] = c.x[at]
        out.y[i] = c.y[at]
        out.value[i] = c.value[at]
        out.weight[i] = c.weight[at]
    end
    return out
end

local function run_hot(c, m, idx, filter_reps, gather_reps)
    local checksum = 0.0
    for r = 1, filter_reps do
        local filtered = filter_cols(c, m)
        checksum = checksum + sum(filtered.value)
    end
    for r = 1, gather_reps do
        local picked = gather_cols(c, idx)
        checksum = checksum + sum(picked.x)
    end
    return checksum
end

local warm = run_hot(cols, mask, indices, 2, 2)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, mask, indices, FILTER_REPS, GATHER_REPS)
local elapsed = os.clock() - t0

print(string.format("soa_filter_gather_hot n=%d filter_reps=%d gather_reps=%d", N, FILTER_REPS, GATHER_REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
