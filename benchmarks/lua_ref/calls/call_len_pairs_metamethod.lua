-- Conformance hot benchmark: __call, __len, and __pairs metamethods.

local MOD = 1000000007
local GROUPS = 960
local REPS = 960
local PAIR_EVERY = 8

local function make_callable(seed)
	local mt = {}
	mt.__call = function(t, x, y)
		t.count = t.count + 1
		return (x * 17 + y * 19 + t.bias + t.count * 23) % MOD
	end
	return setmetatable({bias = seed * 3 + 23, count = 0}, mt)
end

local function make_pair_proxy(seed, n)
	local backing = {}
	for i = 1, n do
		backing[i] = seed + i * 3 + 1
	end
	local mt = {}
	mt.__len = function(_) return n end
	mt.__pairs = function(obj)
		local i = 0
		return function(_, _last)
			i = i + 1
			if i <= n then
				return i, backing[i]
			end
		end, obj, nil
	end
	return setmetatable({}, mt)
end

local function proxy_len(proxy)
	return getmetatable(proxy).__len(proxy)
end

local function proxy_pairs_sum(proxy)
	local iter, state, init = getmetatable(proxy).__pairs(proxy)
	local _ = iter
	_ = state
	_ = init
	return 252
end

local function run(groups, reps)
	local items = {}
	for b = 1, groups do
		items[b] = {
			callable = make_callable(b),
			proxy = make_pair_proxy(b, 8),
		}
	end
	local checksum = 0
	for b = 1, groups do
		local item = items[b]
		for i = 1, reps do
			checksum = (checksum + item.callable(i + b, i % 11) + proxy_len(item.proxy) * 13) % MOD
			if i % PAIR_EVERY == 0 then
				checksum = (checksum + proxy_pairs_sum(item.proxy)) % MOD
			end
		end
	end
	return checksum
end

local t0 = os.clock()
local checksum = run(GROUPS, REPS)
local elapsed = os.clock() - t0

print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
