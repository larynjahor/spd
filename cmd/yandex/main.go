package main

import (
	_ "github.com/larynjahor/spd/data"
	"golang.org/x/tools/go/packages"
)

func main() {
	dr := &packages.DriverRequest{
		Mode:       0,
		Env:        []string{},
		BuildFlags: []string{},
		Tests:      false,
		Overlay:    map[string][]byte{},
	}

	_ = dr

	// env := &pkg.Env{
	// 	Targets: []string{
	// 		"neuro/go",
	// 	},
	// 	Vendor:    true,
	// 	Tags:      []string{},
	// 	GOMOD:     "/Users/larynjahor/arcadia/go.mod",
	// 	GOPATH:    "/Users/larynjahor/go",
	// 	GOROOT:    "/opt/homebrew/Cellar/go/1.24.1/libexec",
	// 	GOARCH:    "",
	// 	GOVERSION: "",
	// 	GOOS:      "",
	// }
}
