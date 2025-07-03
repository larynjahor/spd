package data

import (
	"github.com/larynjahor/spd/pkg/env"
)

var Env = env.Env{
	Targets:   []string{},
	Vendor:    true,
	Tags:      []string{},
	GOMOD:     "root/home/user/pukadia/go.mod",
	GOPATH:    "root/gopath",
	GOROOT:    "root/goroot",
	GOARCH:    "arm64",
	GOVERSION: "go1.24.1",
	GOOS:      "darwin",
}
