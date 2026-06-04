module example.com/leia/examples/database/package-managed
leia 0.1
go 1.25

capability db.open
capability db.query

require github.com/never-labs/leia-db/sqlite v0.1.0

go require modernc.org/sqlite v1.38.2
