print("case:fs_path_glob_cwd_more")

origCwd := fs.cwd()
assert(type(origCwd) == "string" && origCwd != "")
assert(path.isAbs(origCwd))

tmpRoot := fs.tempdir()
assert(type(tmpRoot) == "string" && tmpRoot != "")
assert(path.isAbs(tmpRoot))
assert(fs.exists(tmpRoot))
assert(fs.isdir(tmpRoot))

base := fs.tempfile(nil, "gscript_glob_cwd_")
assert(type(base) == "string" && base != "")
assert(fs.remove(base))
assert(fs.mkdirAll(path.join(base, "nested")))
defer fs.removeAll(base)
defer fs.chdir(origCwd)

assert(path.isAbs(path.abs(base)))
relBase, relErr := path.rel(path.abs(path.dir(base)), path.abs(base))
assert(relErr == nil)
assert(relBase == path.base(base))

assert(fs.writefile(path.join(base, "alpha.txt"), "a"))
assert(fs.writefile(path.join(base, "beta.txt"), "b"))
assert(fs.writefile(path.join(base, "notes.log"), "n"))
assert(fs.writefile(path.join(base, "nested", "gamma.txt"), "g"))

absMatches, absErr := fs.glob(path.join(base, "*.txt"))
assert(absErr == nil)
assert(#absMatches == 2)
assert(path.base(absMatches[1]) == "alpha.txt")
assert(path.base(absMatches[2]) == "beta.txt")

emptyMatches, emptyErr := fs.glob(path.join(base, "*.missing"))
assert(emptyErr == nil)
assert(#emptyMatches == 0)

badMatches, badErr := fs.glob("[")
assert(badMatches == nil)
assert(type(badErr) == "string")

ok, chdirErr := fs.chdir(base)
assert(ok == true)
assert(chdirErr == nil)

cwd := fs.cwd()
assert(path.isAbs(cwd))
dotAbs, dotAbsErr := path.abs(".")
assert(dotAbsErr == nil)
cwdRel, cwdRelErr := path.rel(dotAbs, cwd)
assert(cwdRelErr == nil)
assert(cwdRel == ".")

relativeFile := path.join("nested", "gamma.txt")
assert(!path.isAbs(relativeFile))
relativeAbs, relativeAbsErr := path.abs(relativeFile)
assert(relativeAbsErr == nil)
assert(path.isAbs(relativeAbs))
assert(fs.readfile(relativeAbs) == "g")

relFile, relFileErr := path.rel(cwd, relativeAbs)
assert(relFileErr == nil)
assert(relFile == relativeFile)

relMatches, relGlobErr := fs.glob("*.txt")
assert(relGlobErr == nil)
assert(#relMatches == 2)
assert(relMatches[1] == "alpha.txt")
assert(relMatches[2] == "beta.txt")

nestedMatches, nestedGlobErr := fs.glob(path.join("nested", "*.txt"))
assert(nestedGlobErr == nil)
assert(#nestedMatches == 1)
assert(nestedMatches[1] == relativeFile)

sep := path.separator
listSep := path.listSeparator
assert((sep == "/" || sep == "\\") && #sep == 1)
assert((listSep == ":" || listSep == ";") && #listSep == 1)

missingDir := path.join(base, "missing")
chdirMissing, chdirMissingErr := fs.chdir(missingDir)
assert(chdirMissing == nil)
assert(type(chdirMissingErr) == "string")
assert(path.rel(fs.cwd(), cwd) == ".")

assert(fs.chdir(origCwd))
restoredRel, restoredRelErr := path.rel(origCwd, fs.cwd())
assert(restoredRelErr == nil)
assert(restoredRel == ".")

print("ok")
