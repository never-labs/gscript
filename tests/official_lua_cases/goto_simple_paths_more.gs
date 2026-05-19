print("case:goto_simple_paths_more")

x := nil
y := 12
goto first
second:;
x = x + 1
goto done
first:;
x = y
goto second
done:;
assert(x == 13)

func foo() {
  a := {}
  goto third
first:;
  a[#a + 1] = 1
  goto second
second:;
  a[#a + 1] = 2
  goto fifth
third:;
  a[#a + 1] = 3
  goto first
fourth:;
  a[#a + 1] = 4
  goto sixth
fifth:;
  a[#a + 1] = 5
  goto fourth
sixth:;
  return table.concat(a, ",")
}

assert(foo() == "3,1,2,5,4")

print("ok")
