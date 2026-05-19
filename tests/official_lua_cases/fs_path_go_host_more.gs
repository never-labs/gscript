print("case:fs_path_go_host_more")

base := fs.tempfile(nil, "gscript_official_")
assert(type(base) == "string")
fs.remove(base)
assert(fs.mkdirAll(base .. "/sub"))

file := path.join(base, "sub", "data.txt")
assert(path.base(file) == "data.txt")
assert(path.ext(file) == ".txt")
assert(path.clean(base .. "/sub/../sub/data.txt") == file)

dir, name := path.split(file)
assert(name == "data.txt")
assert(string.find(dir, "sub", 1, true) != nil)
matched, matchErr := path.match("*.txt", "data.txt")
assert(matched == true)
assert(matchErr == nil)
matched, matchErr = path.match("[", "data.txt")
assert(matched == false)
assert(type(matchErr) == "string")

assert(fs.writefile(file, "hello"))
assert(fs.appendfile(file, " world"))
assert(fs.exists(file))
assert(fs.isfile(file))
assert(fs.isdir(base))
assert(fs.readfile(file) == "hello world")

info := fs.stat(file)
assert(info.name == "data.txt")
assert(info.size == 11)
assert(info.isfile == true)

copy := path.join(base, "copy.txt")
assert(fs.copy(file, copy))
assert(fs.readfile(copy) == "hello world")

renamed := path.join(base, "renamed.txt")
assert(fs.rename(copy, renamed))
assert(!fs.exists(copy))
assert(fs.exists(renamed))

entries := fs.readdir(base)
assert(#entries >= 2)

missing, err := fs.readfile(path.join(base, "missing.txt"))
assert(missing == nil)
assert(type(err) == "string")

assert(fs.removeAll(base))
assert(!fs.exists(base))

print("ok")
