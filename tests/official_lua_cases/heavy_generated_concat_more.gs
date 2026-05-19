print("case:heavy_generated_concat_more")

reps := 64
src := "a := \"xy\"; return " .. string.rep("a", reps, " .. ")
f := script.compile(src, {sourceName: "generated-concat.gs"})
s := f()
assert(#s == reps * 2)
assert(string.sub(s, 1, 6) == "xyxyxy")
assert(string.sub(s, -6) == "xyxyxy")

print("ok")
