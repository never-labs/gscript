-- Official hot benchmark: table.move, table.sort, proxy __index/__newindex.

local MOD = 1000000007
local N = 420
local PASSES = 1500

local function hot_move(a1, f, e, t, a2)
	a2 = a2 or a1
	if e >= f then
		if a2 == a1 and t > f and t <= e then
			for i = e, f, -1 do
				a2[t + i - f] = a1[i]
			end
		else
			for i = f, e do
				a2[t + i - f] = a1[i]
			end
		end
	end
	return a2
end

local function checksum_array(a, n)
	local h = 17
	for i = 1, n do
		h = (h * 131 + a[i] * (i % 97 + 1)) % MOD
	end
	return h
end

local function make_array(n, salt)
	local a = {}
	for i = 1, n do
		a[i] = (i * 97 + salt * 53 + n * 17) % 100000
	end
	return a
end

local function run(n, passes)
	local checksum = 0
	for pass = 1, passes do
		local a = make_array(n, pass)
		table.sort(a)
		checksum = (checksum + checksum_array(a, n)) % MOD

		local b = {}
		hot_move(a, 1, n, 1, b)
		hot_move(b, 1, n - 3, 4, b)
		checksum = (checksum + checksum_array(b, n)) % MOD

		local src = b
		local dst = {}
		local reads = 0
		local writes = 0
		local proxySrc = setmetatable({}, {
			__len = function() return n end,
			__index = function(_, k)
				reads = reads + 1
				return src[k]
			end,
		})
		local proxyDst = setmetatable({}, {
			__newindex = function(_, k, v)
				writes = writes + 1
				dst[k] = v
			end,
		})
		hot_move(proxySrc, 1, n, 1, proxyDst)
		local proxyLen = getmetatable(proxySrc).__len(proxySrc)
		checksum = (checksum + checksum_array(dst, n) + reads + writes + proxyLen) % MOD
	end
	return checksum
end

run(80, 2)

local t0 = os.clock()
local checksum = run(N, PASSES)
local elapsed = os.clock() - t0

print(string.format("Checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
