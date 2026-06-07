-- LuaJIT reference for q.operator pipeline hot path: filter, group, aggregate, order, take.

local N = 4096
local REPS = 520
local MOD = 1000000007

local function make_i64(n, mod, offset)
    local t = {}
    for i = 1, n do
        t[i] = offset + (i % mod)
    end
    return t
end

local function make_revenue(n)
    local t = {}
    for i = 1, n do
        t[i] = 90.0 + (i % 149) * 3.25
    end
    return t
end

local function make_cost(n)
    local t = {}
    for i = 1, n do
        t[i] = 25.0 + (i % 67) * 1.15
    end
    return t
end

local function make_units(n)
    local t = {}
    for i = 1, n do
        t[i] = 1.0 + (i % 19)
    end
    return t
end

local function make_risk(n)
    local t = {}
    for i = 1, n do
        t[i] = 0.03 + (i % 41) * 0.0125
    end
    return t
end

local facts = {
    account_id = make_i64(N, 257, 1000),
    segment_id = make_i64(N, 11, 1),
    region_id = make_i64(N, 9, 20),
    revenue = make_revenue(N),
    cost = make_cost(N),
    units = make_units(N),
    risk = make_risk(N),
}

local plan = {
    threshold = 0.25,
}

local function mix(sum, v)
    return (sum * 131 + v) % MOD
end

local function run_hot(cols, p, reps)
    local checksum = 0
    for _ = 1, reps do
        local agg = {}
        for i = 1, N do
            if cols.risk[i] >= p.threshold then
                local segment = cols.segment_id[i]
                local region = cols.region_id[i]
                local key = segment * 100 + region
                local row = agg[key]
                if not row then
                    row = {segment_id = segment, region_id = region, revenue = 0.0, cost = 0.0, units = 0.0, margin = 0.0, max_risk = 0.0, rows = 0}
                    agg[key] = row
                end
                row.revenue = row.revenue + cols.revenue[i]
                row.cost = row.cost + cols.cost[i]
                row.units = row.units + cols.units[i]
                row.margin = row.margin + (cols.revenue[i] - cols.cost[i])
                if cols.risk[i] > row.max_risk then
                    row.max_risk = cols.risk[i]
                end
                row.rows = row.rows + 1
            end
        end

        local rows = {}
        for _, row in pairs(agg) do
            rows[#rows + 1] = row
        end
        table.sort(rows, function(a, b)
            if a.margin == b.margin then
                if a.segment_id == b.segment_id then
                    return a.region_id < b.region_id
                end
                return a.segment_id < b.segment_id
            end
            return a.margin > b.margin
        end)
        local limit = math.min(12, #rows)
        if limit > 0 then
            local top = rows[1]
            local tail = rows[limit]
            checksum = mix(checksum, #rows + top.segment_id * 17 + top.region_id * 31 + top.rows)
            checksum = mix(checksum, math.floor(top.margin) + math.floor(tail.revenue) + math.floor(tail.max_risk * 1000))
        end
    end
    return checksum
end

local warm = run_hot(facts, plan, 2)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(facts, plan, REPS)
local elapsed = os.clock() - t0

print(string.format("q_operator_pipeline_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %d", mix(checksum, warm)))
print(string.format("Time: %.3fs", elapsed))
