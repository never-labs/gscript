print("case:sort_table_insert_errors")

checkerror := func(msg, f, ... ) {
  s, err := pcall(f, ...)
  assert(!s && string.find(err, msg))
}

checkerror("wrong number of arguments", table.insert, {}, 2, 3, 4)
checkerror("bad argument", table.insert)
checkerror("table expected", table.insert, 1, 2)

print("ok")
