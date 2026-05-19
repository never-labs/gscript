print("case:log_go_host_more")

log.clear()
log.setTimestamp(false)
log.setShowLevel(true)
log.setPrefix("svc")

assert(log.DEBUG == 0 && log.INFO == 1 && log.WARN == 2 && log.ERROR == 3 && log.FATAL == 4)
assert(log.format(log.WARN, "disk", 7) == "[WARN] svc disk 7")
log.setLevel(log.WARN)
assert(log.getLevel() == log.WARN)

log.debug("hidden")
log.info("hidden")
log.warn("disk", 7)
log.error("down")

history := log.history()
assert(log.count() == 2)
assert(history[1] == "[WARN] svc disk 7")
assert(history[2] == "[ERROR] svc down")

log.setShowLevel(false)
assert(log.format(log.ERROR, "plain") == "svc plain")
log.clear()
assert(log.count() == 0)

print("ok")
