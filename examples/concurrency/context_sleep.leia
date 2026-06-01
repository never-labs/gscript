ctx, cancel := context.withTimeout(0.005)
_ = cancel

t0 := time.now()
ok, err := time.sleep(ctx, 0.05)
elapsed := time.since(t0)

print(string.format("ok=%s err=%s elapsed_lt_full=%s", tostring(ok), err, tostring(elapsed < 0.04)))
