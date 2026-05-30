-- Conformance hot benchmark: strings and patterns.
-- Covers format, concat, sub/find, match/gmatch/gsub, and long string growth.

local MOD = 1000000007

local function mix(h, v)
	return (h * 131 + v) % MOD
end

local function checksumString(h, s)
	h = mix(h, #s)
	local step = math.floor(#s / 17) + 1
	for i = 1, #s, step do
		h = mix(h, string.byte(s, i))
	end
	if #s > 0 then
		h = mix(h, string.byte(s, #s))
	end
	return h
end

local t0 = os.clock()
local checksum = 17

local rows = {}
for i = 1, 14000 do
	local name = string.format("item_%05d", (i * 37) % 100000)
	local tag = string.format("tag%02d", i % 97)
	local line = string.format("%s;%s;value=%06d;hex=%04x", name, tag, i * 13, (i * 17) % 65536)
	rows[i] = line
	checksum = checksumString(checksum, line)
end

local blob = table.concat(rows, "\n")
checksum = checksumString(checksum, blob)

local subFindTotal = 0
for i = 1, 22000 do
	local start = ((i * 29) % (#blob - 120)) + 1
	local part = string.sub(blob, start, start + 89)
	subFindTotal = subFindTotal + #part
	local a, b = string.find(part, "value=", 1, true)
	if a ~= nil then
		subFindTotal = subFindTotal + a + b
	end
	a, b = string.find(part, "tag%d%d")
	if a ~= nil then
		subFindTotal = subFindTotal + a * 3 + b
	end
	if i % 7 == 0 then
		checksum = checksumString(checksum, part)
	end
end
checksum = mix(checksum, subFindTotal)

local patternTotal = 0
for _ = 1, 18 do
	for item, value in string.gmatch(blob, "item_(%d+);tag%d%d;value=(%d+)") do
		patternTotal = patternTotal + tonumber(string.sub(item, 4, 5))
		patternTotal = patternTotal + tonumber(string.sub(value, 5, 6))
	end
end
checksum = mix(checksum, patternTotal)

local matchTotal = 0
for i = 1, 26000 do
	local idx = ((i * 19) % #rows) + 1
	local item, value = string.match(rows[idx], "(item_%d+);tag%d%d;value=(%d+)")
	matchTotal = matchTotal + #item + tonumber(string.sub(value, 4, 6))
end
checksum = mix(checksum, matchTotal)

local function repl(num, tag)
	return tag .. ":" .. num
end

local rewrite = ""
for _ = 1, 10 do
	rewrite = string.gsub(blob, "item_(%d+);(tag%d%d)", repl)
	checksum = checksumString(checksum, rewrite)
end

local grow = ""
for i = 1, 9000 do
	grow = grow .. string.sub(rows[(i % #rows) + 1], 1, 8)
	if i % 900 == 0 then
		checksum = checksumString(checksum, grow)
	end
end
checksum = checksumString(checksum, grow)

local finalLen = #blob + #rewrite + #grow
checksum = mix(checksum, finalLen)
local elapsed = os.clock() - t0

print(string.format("checksum: %d", checksum))
print(string.format("Time: %.3fs", elapsed))
