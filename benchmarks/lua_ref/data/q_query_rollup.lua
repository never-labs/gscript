-- LuaJIT reference for q.query rollup hot path: grouped rollup, ordering, limit.

local N = 4096
local REPS = 450
local MOD = 1000000007

local function make_channel(n)
    local t = {}
    for i = 1, n do
        t[i] = 1 + (i % 17)
    end
    return t
end

local function make_region(n)
    local t = {}
    for i = 1, n do
        t[i] = 1 + (i % 7)
    end
    return t
end

local function make_revenue(n)
    local t = {}
    for i = 1, n do
        t[i] = 40.0 + (i % 97) * 1.25
    end
    return t
end

local function make_cost(n)
    local t = {}
    for i = 1, n do
        t[i] = 15.0 + (i % 43) * 0.75
    end
    return t
end

local function make_units(n)
    local t = {}
    for i = 1, n do
        t[i] = 1.0 + (i % 11)
    end
    return t
end

local campaigns = {
    channel_id = make_channel(N),
    region_id = make_region(N),
    revenue = make_revenue(N),
    cost = make_cost(N),
    units = make_units(N),
}

local active = {}
for i = 1, N do
    active[i] = campaigns.revenue[i] >= 80.0
end

local function mix(sum, v)
    return (sum * 131 + v) % MOD
end

local function run_hot(cols, mask, reps)
    local checksum = 0
    for _ = 1, reps do
        local agg = {}
        for i = 1, N do
            if mask[i] then
                local channel = cols.channel_id[i]
                local region = cols.region_id[i]
                local key = channel * 100 + region
                local row = agg[key]
                if not row then
                    row = {channel_id = channel, region_id = region, revenue = 0.0, cost = 0.0, units = 0.0, rows = 0}
                    agg[key] = row
                end
                row.revenue = row.revenue + cols.revenue[i]
                row.cost = row.cost + cols.cost[i]
                row.units = row.units + cols.units[i]
                row.rows = row.rows + 1
            end
        end

        local rows = {}
        for _, row in pairs(agg) do
            row.gross = row.revenue - row.cost
            rows[#rows + 1] = row
        end
        table.sort(rows, function(a, b)
            if a.gross == b.gross then
                if a.channel_id == b.channel_id then
                    return a.region_id < b.region_id
                end
                return a.channel_id < b.channel_id
            end
            return a.gross > b.gross
        end)
        local limit = math.min(8, #rows)
        if limit > 0 then
            local top = rows[1]
            checksum = mix(checksum, #rows + top.channel_id * 17 + top.region_id * 31 + top.rows)
            checksum = mix(checksum, math.floor(top.gross))
            if limit > 1 then
                local second = rows[2]
                checksum = mix(checksum, math.floor(second.gross))
            end
        end
    end
    return checksum
end

local warm = run_hot(campaigns, active, 2)
collectgarbage("collect")
local t0 = os.clock()
local checksum = run_hot(campaigns, active, REPS)
local elapsed = os.clock() - t0

print(string.format("q_query_rollup_hot n=%d reps=%d", N, REPS))
print(string.format("checksum: %d", mix(checksum, warm)))
print(string.format("Time: %.3fs", elapsed))
