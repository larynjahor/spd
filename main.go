package main

import (
	"os"
	"runtime/pprof"
	"slices"
	"strings"

	_ "net/http/pprof"

	"github.com/goccy/go-json"
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

	if !slices.ContainsFunc(os.Args[1:], func(p string) bool { return strings.Contains(p, "/arcadia/...") }) {
		dr.NotHandled = true

		writeResponse(&dr)
		return
	}

	targets := []string{
		"/Users/larynjahor/arcadia/yy/backend",
		"/Users/larynjahor/arcadia/yy/yaart-api/backend",
		"/Users/larynjahor/arcadia/neuro/go",
		"/Users/larynjahor/arcadia/neuro/suggest",
		"/Users/larynjahor/arcadia/neuroexpert/backend",
		"/Users/larynjahor/arcadia/thefeed/backend",
		"/Users/larynjahor/arcadia/browser/backend/extra/summary-bot",
		"/Users/larynjahor/arcadia/library/go",
	}

	p, err := parser.New()
	if err != nil {
		panic(err)
	}

	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		panic(err)
	}

	dr.GoVersion = p.Env.MinorVersion()
	dr.Arch = p.Env.GOARCH
	dr.Compiler = "gc"

	dr.Packages, err = p.ParseTargets(targets)
	if err != nil {
		panic(err)
	}

	for _, p := range dr.Packages {
		if strings.Contains(p.ID, "yandex-team") {
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
