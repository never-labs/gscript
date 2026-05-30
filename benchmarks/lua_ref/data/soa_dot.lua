-- Data-oriented hot benchmark reference: LuaJIT dense and masked dot products.

local N = 65536
local REPS = 900

local function make_f64(n, scale, bias)
    local t = {}
    for i = 1, n do
        t[i] = i * scale + bias
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
    x = make_f64(N, 0.001, 1.0),
    y = make_f64(N, 0.002, 0.5),
    weight = make_f64(N, 0.01, 0.25),
}
local mask = make_mask(N)

local function dot(a, b)
    local sum = 0.0
    for i = 1, #a do
        sum = sum + a[i] * b[i]
    end
    return sum
end

local function dot_where(a, b, m)
    local sum = 0.0
    for i = 1, #m do
        if m[i] then
            sum = sum + a[i] * b[i]
        end
    end
    return sum
end

local function run_hot(c, m, reps)
    local checksum = 0.0
    for r = 1, reps do
        checksum = checksum + dot(c.x, c.y)
        checksum = checksum + dot_where(c.x, c.weight, m)
    end
    return checksum
end

local warm = run_hot(cols, mask, 2)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, mask, REPS)
local elapsed = os.clock() - t0

print(string.format("soa_dot_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
