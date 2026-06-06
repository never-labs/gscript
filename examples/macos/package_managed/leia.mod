module example.com/leia/examples/macos/package-managed
leia 0.1
go 1.25

capability macos.automation
capability process.exec

require github.com/never-labs/leia-macos/automation v0.1.0
replace github.com/never-labs/leia-macos/automation v0.1.0 => ./adapter

go require github.com/never-labs/leia-macos/automation v0.1.0
go require github.com/never-labs/leia v0.0.0-20260601065425-1c9cadbd856f
go replace github.com/never-labs/leia-macos/automation v0.1.0 => ./adapter
