print("case:constructs_label_goto_control")

local i = 0
local sum = 0
while true do
	i = i + 1
	if i > 6 then
		break
	end
	if i ~= 2 then
		sum = sum + i
	end
end

print("sum", sum)

local escaped = "no"
while true do
	escaped = "yes"
	break
end
print("escaped", escaped)

local function collect(n)
	local out = ""
	while n > 0 do
		out = out .. tostring(n)
		n = n - 1
	end
	return out
end

print("collect", collect(4))
