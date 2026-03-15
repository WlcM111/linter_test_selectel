module github.com/WlcM111/linter_test_selectel

go 1.23.0

toolchain go1.23.2

require (
	github.com/golangci/plugin-module-register v0.1.2
	golang.org/x/tools v0.32.0
)

replace github.com/golangci/plugin-module-register => ./third_party/github.com/golangci/plugin-module-register

replace golang.org/x/tools => ./third_party/golang.org/x/tools
