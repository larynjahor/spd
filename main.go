package main

import (
	"fmt"
	"os"
	"path"
	"runtime/pprof"
	"slices"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/goccy/go-json"
	"github.com/ijimiji/yolist/internal/parser"
	"golang.org/x/tools/go/packages"
)

var profile = true

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

		// mem, err := os.Create("mem.prof")
		// if err != nil {
		// 	panic(err)
		// }
		//
		// defer mem.Close()
		//
		// runtime.GC()
		//
		// if err := pprof.WriteHeapProfile(mem); err != nil {
		// 	panic(err)
		// }
	}

	var (
		err error
		req packages.DriverRequest
		dr  DriverResponse
	)

	os.WriteFile(path.Join("/Users/larynjahor/gits/yolist", fmt.Sprint(time.Now().Unix())), []byte(fmt.Sprint(os.Args)), os.ModePerm)

	if !slices.ContainsFunc(os.Args[1:], func(p string) bool { return strings.Contains(p, "/arcadia/...") }) {
		dr.NotHandled = true

		exit(&dr)
	}

	targets := []string{
		"/Users/larynjahor/arcadia/yy/backend",
		"/Users/larynjahor/arcadia/yy/yaart-api/backend",
		"/Users/larynjahor/arcadia/neuro/go",
		"/Users/larynjahor/arcadia/neuro/suggest",
		"/Users/larynjahor/arcadia/neuroexpert/backend",
		"/Users/larynjahor/arcadia/thefeed/backend",
		"/Users/larynjahor/arcadia/browser/backend/extra/summary-bot",
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

	exit(&dr)
}

func exit(dr *DriverResponse) {
	if err := json.NewEncoder(os.Stdout).Encode(dr); err != nil {
		panic(err)
	}

	pprof.StopCPUProfile()

	os.Exit(0)
}

type DriverResponse struct {
	NotHandled bool

	Compiler string
	Arch     string

	Roots []string `json:",omitempty"`

	Packages []*parser.Package

	GoVersion int
}
