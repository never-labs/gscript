print("case:csv_go_host_more")

rows := csv.parse("name,score\n\"a,b\",10\n c ,20\n", {trimSpace: true})
assert(#rows == 3)
assert(rows[1][1] == "name")
assert(rows[2][1] == "a,b")
assert(rows[2][2] == "10")
assert(rows[3][1] == "c ")

semi := csv.parse("a;b\n1;2\n", {sep: ";"})
assert(semi[2][1] == "1")
assert(semi[2][2] == "2")

withHeaders := csv.parseWithHeaders("name,score\nalice,10\nbob,20\n")
assert(#withHeaders == 2)
assert(withHeaders[1].name == "alice")
assert(withHeaders[2].score == "20")

encoded := csv.encode({{"a", "b"}, {"1", "2"}})
assert(encoded == "a,b\n1,2\n")

encoded = csv.encodeWithHeaders({{name: "alice", score: "10"}}, {"name", "score"}, {sep: ";"})
assert(encoded == "name;score\nalice;10\n")

ok, err := pcall(csv.parse, "\"unterminated")
assert(ok == false)
assert(type(err) == "string")

print("ok")
