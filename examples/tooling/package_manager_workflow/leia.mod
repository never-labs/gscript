module example.com/leia/examples/tooling/package-manager-workflow
leia 0.1
go 1.25

capability fs.read
capability process.exec
capability module.graph
capability module.verify
capability module.capability

require github.com/never-labs/leia-package-workflow/metadata v0.2.0
replace github.com/never-labs/leia-package-workflow/metadata v0.2.0 => ./local/metadata

go require github.com/never-labs/leia v0.0.0-20260601065425-1c9cadbd856f
go require github.com/never-labs/leia-package-workflow/metadata v0.2.0
go replace github.com/never-labs/leia-package-workflow/metadata v0.2.0 => ./local/metadata
