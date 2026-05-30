print("case:main_script_process_more")

sum := script.eval("return a + b", {env: {a: 7, b: 5}, sourceName: "generated/sum.gs"})
assert(sum == 12)

fn := script.compile("return name", script.sandbox({name: "sandboxed"}))
assert(fn() == "sandboxed")

oldDir := script.dir()
assert(type(oldDir) == "string")
assert(script.setDir(oldDir) == oldDir)

process.setArgs("tool.gs", "build", "--fast")
args := process.args()
entry := process.entry()
assert(args[0] == "tool.gs" && args[1] == "build" && args[2] == "--fast")
assert(entry.file == "tool.gs")
assert(entry.args[1] == "build")
assert(type(process.pid()) == "number")

print("ok")
