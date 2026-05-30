-- Data-oriented hot benchmark reference: LuaJIT column-array affine updates.

local N = 262144
local STEPS = 300

local function make_f64(n, scale, bias)
    local t = {}
    for i = 1, n do
        t[i] = i * scale + bias
    end
    return t
end

local cols = {
    x = make_f64(N, 0.001, 0.0),
    y = make_f64(N, 0.002, 1.0),
    z = make_f64(N, 0.003, 2.0),
    vx = make_f64(N, 0.00001, 0.01),
    vy = make_f64(N, 0.00002, -0.02),
    vz = make_f64(N, -0.00001, 0.015),
}

local function sum(a)
    local s = 0.0
    for i = 1, #a do
        s = s + a[i]
    end
    return s
end

local function run_hot(c, steps, dt)
    local x, y, z = c.x, c.y, c.z
    local vx, vy, vz = c.vx, c.vy, c.vz
    for step = 1, steps do
        for i = 1, #x do
            x[i] = vx[i] * dt + 0.0
            y[i] = vy[i] * dt + 1.0
            z[i] = vz[i] * dt + 2.0
        end
    end
    return sum(x) + sum(y) + sum(z)
end

local warm = run_hot(cols, 3, 0.016)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(cols, STEPS, 0.016)
local elapsed = os.clock() - t0

print(string.format("soa_affine_many_hot n=%d steps=%d", N, STEPS))
print(string.format("checksum: %.6f", checksum + warm * 0.000001))
print(string.format("Time: %.3fs", elapsed))
