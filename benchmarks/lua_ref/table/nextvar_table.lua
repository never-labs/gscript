-- Conformance hot benchmark: nextvar/table/api/gc traversal family.
-- Matches benchmarks/table/nextvar_table.leia.

local MOD = 1000000007

if rawlen == nil then
	function rawlen(t)
		return #t
	end
end

local function addmod(a, b, ...)
    return (a + b) % MOD
end

local function build_mixed(n, ...)
    local t = {}
    for i = 1, n do
        t[i] = i * 3 + 1
        if i % 3 == 0 then
            t["k" .. i] = i * 7 + 5
        end
        if i % 10 == 0 then
            t[-i] = i * 11 + 9
        end
    end
    return t
end

local function scan_pairs(t, ...)
    local sum = 0
    local count = 0
    for k, v in pairs(t) do
        if type(k) == "number" then
            sum = addmod(sum, k * 3 + v)
        else
            sum = addmod(sum, #k * 5 + v)
        end
        count = count + 1
    end
    return addmod(sum, count * 17)
end

local function scan_next(t, ...)
    local sum = 0
    local count = 0
    local k = nil
    local v = nil
    while true do
        k, v = next(t, k)
        if k == nil then
            break
        end
        if type(k) == "number" then
            sum = addmod(sum, k * 13 + v)
        else
            sum = addmod(sum, #k * 19 + v)
        end
        count = count + 1
    end
    return addmod(sum, count * 23)
end

local function scan_ipairs(t, ...)
    local sum = 0
    local count = 0
    for i, v in ipairs(t) do
        sum = addmod(sum, i * 29 + v)
        count = count + 1
    end
    return addmod(sum, count * 31)
end

local function mutate_table(n, reps, ...)
    local t = {}
    for i = 1, n do
        t[i] = i
        rawset(t, "s" .. i, i + 1)
    end

    local checksum = addmod(rawlen(t), #t)
    for r = 1, reps do
        local pos = (r % n) + 1
        table.insert(t, pos, r)
        local removed = table.remove(t, pos + 1)
        local hotKey = "hot" .. (r % 64)
        rawset(t, hotKey, removed)
        if r % 5 == 0 then
            rawset(t, n + 8, r)
            rawset(t, n + 8, nil)
        end
        checksum = addmod(checksum, rawget(t, hotKey) + rawlen(t) + #t)
    end
    return checksum
end

local function allocation_pressure(n, reps, ...)
    local roots = {}
    local checksum = 0
    for r = 1, reps do
        local batch = {}
        local prev = nil
        for i = 1, n do
            local obj = {
                id = i + r,
                tag = "node",
                value = (i * r) % 997,
                left = prev,
                right = nil,
            }
            if prev ~= nil then
                prev.right = obj
            end
            batch[i] = obj
            prev = obj
        end
        for i = 1, n, 4 do
            local obj = batch[i]
            checksum = addmod(checksum, obj.id + obj.value)
        end
        roots[(r % 32) + 1] = batch
    end
    return addmod(checksum, #roots * 37)
end

local function run_once(size, reps, allocN, allocReps, ...)
    local checksum = 0
    for _ = 1, reps do
        local t = build_mixed(size)
        checksum = addmod(checksum, scan_pairs(t))
        checksum = addmod(checksum, scan_next(t))
        checksum = addmod(checksum, scan_ipairs(t))
    end
    checksum = addmod(checksum, mutate_table(size, reps * 6))
    checksum = addmod(checksum, allocation_pressure(allocN, allocReps))
    return checksum
end

local SIZE = 2600
local REPS = 90
local ALLOC_N = 700
local ALLOC_REPS = 180

local warm = run_once(300, 4, 120, 8)

local t0 = os.clock()
local checksum = run_once(SIZE, REPS, ALLOC_N, ALLOC_REPS)
local elapsed = os.clock() - t0

print(string.format("Checksum: %d", checksum + warm - warm))
print(string.format("Time: %.3fs", elapsed))
