-- Official hot benchmark: events/metamethod dispatch.
-- Covers __index/__newindex, method dispatch, arithmetic/compare, and concat.

local MOD = 1000000007
local N = 600000

local methods = {}

local function proxy_index(obj, key)
    if key == "step" then
        return methods.step
    end
    if key == "mix" then
        return methods.mix
    end
    local slots = rawget(obj, "slots")
    local value = slots[key]
    if value ~= nil then
        return value
    end
    return rawget(obj, "base") + #key
end

local function proxy_newindex(obj, key, value)
    local slots = rawget(obj, "slots")
    slots[key] = value
end

local proxy_mt = {
    __index = proxy_index,
    __newindex = proxy_newindex,
}

methods.step = function(self, delta)
    self.accum = self.accum + delta + self.bias
    return self.accum
end

methods.mix = function(self, i)
    return self:step(i % 17) + self.shadow
end

local function new_proxy(base)
    local obj = {base = base, slots = {accum = 0, bias = 3, shadow = 11}}
    return setmetatable(obj, proxy_mt)
end

local num_mt = {}

local function new_num(v)
    return setmetatable({v = v}, num_mt)
end

num_mt.__add = function(a, b)
    return a.v + b.v
end

num_mt.__sub = function(a, b)
    return a.v - b.v
end

num_mt.__mul = function(a, b)
    return a.v * b.v
end

num_mt.__unm = function(a)
    return -a.v
end

num_mt.__lt = function(a, b)
    return a.v < b.v
end

num_mt.__le = function(a, b)
    return a.v <= b.v
end

local str_mt = {}

str_mt.__concat = function(a, b)
    if type(a) == "table" then
        a = a.s
    end
    if type(b) == "table" then
        b = b.s
    end
    return a .. b
end

local function new_str(s)
    return setmetatable({s = s}, str_mt)
end

local function run_events(n)
    local p = new_proxy(23)
    local a = new_num(7)
    local b = new_num(5)
    local c = new_num(12)
    local sa = new_str("a")
    local sb = new_str("b")
    local checksum = 0

    for i = 1, n do
        local v = p:mix(i)
        checksum = (checksum + v + p.missing + p.accum) % MOD

        local arith = (a + b) * (c - b) + (-b)
        if arith ~= nil then
            checksum = (checksum + 13) % MOD
        end
        if a < c then
            checksum = (checksum + i % 97) % MOD
        end
        if b <= a then
            checksum = (checksum + 31) % MOD
        end
        if i % 16 == 0 then
            local s = sa .. sb
            checksum = (checksum + #s) % MOD
        end
    end

    return checksum
end

local warm = run_events(2000)

local t0 = os.clock()
local checksum = (run_events(N) + warm) % MOD
local elapsed = os.clock() - t0

print(string.format("events_metamethod_hot(%d): checksum: %d", N, checksum))
print(string.format("Time: %.3fs", elapsed))
