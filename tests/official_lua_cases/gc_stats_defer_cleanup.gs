print("case:gc_stats_defer_cleanup")

func checkStats() {
  stats := collectgarbage("stats")
  assert(type(stats.allocBytes) == "number")
  assert(type(stats.allocKB) == "number")
  assert(type(stats.sysBytes) == "number")
  assert(type(stats.heapObjects) == "number")
  assert(type(stats.numGC) == "number")
  assert(type(stats.rootLog) == "number")
  assert(type(stats.running) == "boolean")
  assert(type(stats.mode) == "string")
  return stats.running, stats.mode
}

running, mode := checkStats()
assert(type(running) == "boolean")
assert(type(mode) == "string")

before := collectgarbage("stats")
collectgarbage("collect")
after := collectgarbage("stats")
assert(after.numGC >= before.numGC)

print("ok")
