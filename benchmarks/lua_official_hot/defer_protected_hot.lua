-- Official hot benchmark: defer coverage, protected calls, and cached coroutine helpers.
-- Uses deterministic logical timing instead of wall time.

local MOD = 1000000007
local DEFER_N = 64
local PROTECTED_N = 180000
local COROUTINE_N = 45000

local deferTotal = 0
local logicalTime = 0.0

local function mix(h, v)
	return (h * 131 + v) % MOD
end

local function addWork(units, cost)
	logicalTime = logicalTime + units * cost
end

local function record(x)
	deferTotal = mix(deferTotal, x)
end

local function with_defer(i)
	local localv = i % 97
	local ok, result = pcall(function()
		if i % 5 == 0 then
			error("boom")
		end
		return localv * 7 + 11
	end)
	record(localv + 3)
	record(localv + 1)
	if not ok then
		error(result)
	end
	return result
end

local function defer_probe(n)
	local sum = 0
	for i = 1, n do
		local ok, value = pcall(with_defer, i)
		if ok then
			sum = mix(sum, value)
		else
			sum = mix(sum, i * 13)
		end
	end
	addWork(n, 0.000004)
	return mix(sum, deferTotal)
end

local function protected_body(i)
	local v = i % 97
	if i % 23 == 0 then
		error("protected-boom")
	end
	return v * 7 + 11
end

local function protected_hot(n)
	local sum = 0
	for i = 1, n do
		local ok, value = pcall(protected_body, i)
		if ok then
			sum = (sum + value) % MOD
		else
			sum = (sum + i * 13) % MOD
		end
	end
	addWork(n, 0.000001)
	return sum
end

local cachedYield = coroutine.yield

local function coroutine_hot(n)
	local co = coroutine.create(function(seed)
		local acc = seed
		for i = 1, n do
			cachedYield(acc)
			acc = (acc + i * 17) % MOD
		end
		return acc
	end)
	local ok, value = coroutine.resume(co, 5)
	if not ok then
		error(value)
	end
	local sum = 0
	for i = 1, n - 1 do
		ok, value = coroutine.resume(co)
		if not ok then
			error(value)
		end
		sum = (sum + i * 19) % MOD
	end
	addWork(n, 0.000003)
	return sum
end

local checksum = 17
local deferChecksum = defer_probe(DEFER_N)
local protectedChecksum = protected_hot(PROTECTED_N)
local coroutineChecksum = coroutine_hot(COROUTINE_N)
checksum = mix(checksum, deferChecksum)
checksum = mix(checksum, protectedChecksum)
checksum = mix(checksum, coroutineChecksum)
checksum = mix(checksum, deferTotal)

print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", logicalTime))
