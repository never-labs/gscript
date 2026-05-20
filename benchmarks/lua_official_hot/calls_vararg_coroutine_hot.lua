-- Hot benchmark: calls, varargs, closures, coroutine resume/yield control flow.

local MOD = 1000000007
local N_CALLS = 220000
local N_CORO = 90000

if table.pack == nil then
	function table.pack(...)
		return {n = select("#", ...), ...}
	end
end
if table.unpack == nil then
	table.unpack = unpack
end

local function triple(x)
	return x, x + 1, x + 2
end

local function adjusted_call(i)
	local a = i
	local b = i + 1
	local c = i + 2
	local d = i + 5
	return (a + b * 3 + c * 5 + d * 7 + i * 11) % MOD
end

local function vararg_fold(base, ...)
	local n = select("#", ...)
	local u1 = select(1, ...)
	local u2 = select(2, ...)
	local u3 = select(3, ...)
	local u4 = select(4, ...)
	local tail = select(4, ...)
	return (base + n * 13 + u1 * 17 + u2 * 19 + u3 * 23 + u4 * 29 + tail * 31) % MOD
end

local function make_worker(seed)
	local captured = seed
	return function(i, v1, v2, v3, v4)
		local a = i + captured
		local b = a + 1
		local c = a + 2
		local v = (a + v1 * 17 + v2 * 19 + v3 * 23 + v4 * 29) % MOD
		captured = (captured + b + c + 4) % MOD
		return (v + captured) % MOD
	end
end

local function coroutine_pipeline(worker, n)
	local co = coroutine.create(function(start)
		local acc = start
		for i = 1, n do
			coroutine.yield(acc)
			local step = i % 11
			local a = i + 1
			local b = i + 2
			local c = i + 3
			acc = (acc + worker(i + step, a, b, c, step) + step) % MOD
		end
		return acc, n
	end)

	local ok, pair = coroutine.resume(co, 7)
	if not ok then error(pair) end

	local total = 0
	for i = 1, n - 1 do
		ok, pair = coroutine.resume(co)
		if not ok then error(pair) end
		total = (total + i * 17) % MOD
	end
	return total
end

local function workload(nCalls, nCoro)
	local worker = make_worker(19)
	local checksum = 0
	for i = 1, nCalls do
		checksum = (checksum + adjusted_call(i)) % MOD
		checksum = (checksum + vararg_fold(i, i + 1, i + 2, i + 3, i + 4)) % MOD
		checksum = (checksum + worker(i, i + 1, i + 2, i + 3, i + 4)) % MOD
	end
	checksum = (checksum + coroutine_pipeline(worker, nCoro)) % MOD
	return checksum
end

workload(2000, 1000)

local t0 = os.clock()
local checksum = workload(N_CALLS, N_CORO)
local elapsed = os.clock() - t0

print(string.format("calls_vararg_coroutine_hot checksum=%d", checksum))
print(string.format("Time: %.3fs", elapsed))
