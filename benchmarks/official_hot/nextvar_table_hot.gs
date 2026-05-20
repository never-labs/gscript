// Official hot benchmark: nextvar/table/api/gc traversal family.
// Stresses pairs/next/ipairs, mixed numeric/string keys, insert/remove,
// raw helpers, length boundaries, and allocation pressure.

MOD := 1000000007

func addmod(a, b, ...) {
    return (a + b) % MOD
}

func build_mixed(n, ...) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i * 3 + 1
        if i % 3 == 0 {
            t["k" .. i] = i * 7 + 5
        }
        if i % 10 == 0 {
            t[-i] = i * 11 + 9
        }
    }
    return t
}

func scan_pairs(t, ...) {
    sum := 0
    count := 0
    for k, v := range pairs(t) {
        if type(k) == "number" {
            sum = addmod(sum, k * 3 + v)
        } else {
            sum = addmod(sum, #k * 5 + v)
        }
        count = count + 1
    }
    return addmod(sum, count * 17)
}

func scan_next(t, ...) {
    sum := 0
    count := 0
    k := nil
    v := nil
    for {
        k, v = next(t, k)
        if k == nil {
            break
        }
        if type(k) == "number" {
            sum = addmod(sum, k * 13 + v)
        } else {
            sum = addmod(sum, #k * 19 + v)
        }
        count = count + 1
    }
    return addmod(sum, count * 23)
}

func scan_ipairs(t, ...) {
    sum := 0
    count := 0
    for i, v := range ipairs(t) {
        sum = addmod(sum, i * 29 + v)
        count = count + 1
    }
    return addmod(sum, count * 31)
}

func mutate_table(n, reps, ...) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i
        rawset(t, "s" .. i, i + 1)
    }

    checksum := addmod(rawlen(t), #t)
    for r := 1; r <= reps; r++ {
        pos := (r % n) + 1
        table.insert(t, pos, r)
        removed := table.remove(t, pos + 1)
        hotKey := "hot" .. (r % 64)
        rawset(t, hotKey, removed)
        if r % 5 == 0 {
            rawset(t, n + 8, r)
            rawset(t, n + 8, nil)
        }
        checksum = addmod(checksum, rawget(t, hotKey) + rawlen(t) + #t)
    }
    return checksum
}

func allocation_pressure(n, reps, ...) {
    roots := {}
    checksum := 0
    for r := 1; r <= reps; r++ {
        batch := {}
        prev := nil
        for i := 1; i <= n; i++ {
            obj := {
                id: i + r,
                tag: "node",
                value: (i * r) % 997,
                left: prev,
                right: nil,
            }
            if prev != nil {
                prev.right = obj
            }
            batch[i] = obj
            prev = obj
        }
        for i := 1; i <= n; i = i + 4 {
            obj := batch[i]
            checksum = addmod(checksum, obj.id + obj.value)
        }
        roots[(r % 32) + 1] = batch
    }
    return addmod(checksum, #roots * 37)
}

func run_once(size, reps, allocN, allocReps, ...) {
    checksum := 0
    for r := 1; r <= reps; r++ {
        t := build_mixed(size)
        checksum = addmod(checksum, scan_pairs(t))
        checksum = addmod(checksum, scan_next(t))
        checksum = addmod(checksum, scan_ipairs(t))
    }
    checksum = addmod(checksum, mutate_table(size, reps * 6))
    checksum = addmod(checksum, allocation_pressure(allocN, allocReps))
    return checksum
}

SIZE := 2600
REPS := 90
ALLOC_N := 700
ALLOC_REPS := 180

warm := run_once(300, 4, 120, 8)

t0 := time.now()
checksum := run_once(SIZE, REPS, ALLOC_N, ALLOC_REPS)
elapsed := time.since(t0)

print(string.format("Checksum: %d", checksum + warm - warm))
print(string.format("Time: %.3fs", elapsed))
