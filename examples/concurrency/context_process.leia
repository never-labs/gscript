ctx, cancel := context.withTimeout(0.01)
_ = cancel

result := process.run(ctx, {"sh", "-c", "sleep 1; echo late"})

print(string.format("ok=%s cancelled=%s err=%s late=%s",
    tostring(result.ok),
    tostring(result.cancelled),
    result.err,
    tostring(string.find(result.stdout, "late") != nil)))
