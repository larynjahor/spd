package main

import (
	"os"
	"runtime/pprof"
	"strings"

	_ "net/http/pprof"

	"github.com/goccy/go-json"
	"github.com/ijimiji/yolist/internal/config"
	"github.com/ijimiji/yolist/internal/parser"
	"golang.org/x/tools/go/packages"
)

var profile = false

func main() {
	if profile {
		cpu, err := os.Create("cpu.prof")
		if err != nil {
			panic(err)
		}

		defer cpu.Close()

		if err := pprof.StartCPUProfile(cpu); err != nil {
			panic(err)
		}
	}

	var (
		err error
		req packages.DriverRequest
		dr  DriverResponse
	)

	cfg := config.Load()

	var contains bool
	for pattern := range cfg.Patterns {
		for _, arg := range os.Args[1:] {
			if strings.Contains(arg, pattern) {
				contains = true
			}
		}
	}

	if !contains {
		dr.NotHandled = true

		writeResponse(&dr)
		return
	}

	var targets []string

	for _, files := range cfg.Patterns {
		targets = append(targets, files...)
	}

	p, err := parser.New(targets)
	if err != nil {
		panic(err)
	}

	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		panic(err)
	}

	env := p.Env()

	dr.GoVersion = env.MinorVersion()
	dr.Arch = env.GOARCH
	dr.Compiler = "gc"

	dr.Packages, err = p.Packages()
	if err != nil {
		panic(err)
	}

	for _, p := range dr.Packages {
		if !p.DepOnly && strings.HasSuffix(p.ID, "main") {
			dr.Roots = append(dr.Roots, p.ID)
		}
	}

	dr.Roots = append(dr.Roots, "builtin")

	writeResponse(&dr)

	pprof.StopCPUProfile()
}

func writeResponse(dr *DriverResponse) {
	if err := json.NewEncoder(os.Stdout).Encode(dr); err != nil {
		panic(err)
	}
}

type DriverResponse struct {
	NotHandled bool

	Compiler string
	Arch     string

	Roots []string `json:",omitempty"`

	Packages []*parser.Package

	GoVersion int
}
