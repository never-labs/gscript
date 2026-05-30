-- Conformance hot benchmark: regexp-like submatches, findAll/split, and deterministic random intervals.

local MOD = 1000000007
local N = 36000

local function mix(h, v)
	return (h * 131 + v) % MOD
end

local function line(i)
	return string.format("svc=api%d status=%d route=/v1/items/%d trace=t%05d", i % 17, 200 + (i % 5) * 100, i % 997, (i * 37) % 100000)
end

local function split_space(s)
	local out = {}
	for part in string.gmatch(s, "%S+") do
		out[#out + 1] = part
	end
	return out
end

local function run(n)
	local checksum = 0
	local seed = 17
	local seen = {}
	local intervalHits = 0
	for i = 1, n do
		local s = line(i)
		do
			local key, value = string.match(s, "([a-z]+)=([a-z0-9/]+)")
			local whole = key .. "=" .. value
			checksum = mix(checksum, #whole + #key * 3 + #value * 7)
		end
		for key, value in string.gmatch(s, "([a-z]+)=([a-z0-9/]+)") do
			local whole = key .. "=" .. value
			checksum = mix(checksum, #whole + #key + #value)
		end
		local numCount = 0
		for num in string.gmatch(s, "[0-9]+") do
			numCount = numCount + 1
			if numCount <= 3 then
				checksum = mix(checksum, tonumber(num) % 10007)
			end
		end
		local parts = split_space(s)
		checksum = mix(checksum, #parts)

		seed = (seed * 48271) % 2147483647
		local r = (seed % 97) - 48
		seen[r] = true
		checksum = mix(checksum, r + 100)

		local width = (i % 31) + 1
		local low = (seed % 200) - 100
		local high = low + width
		local pick = low + (seed % (width + 1))
		if pick >= low and pick <= high then
			intervalHits = intervalHits + 1
		end
		checksum = mix(checksum, (high - low) * 5 + pick + 128)
	end
	local count = 0
	for _ in pairs(seen) do
		count = count + 1
	end
	return mix(mix(checksum, count), intervalHits)
end

run(1000)

local t0 = os.clock()
local checksum = run(N)
local elapsed = os.clock() - t0

print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
