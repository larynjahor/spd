package main

import (
	"os"
	"path"
	"runtime/pprof"
	"time"

	_ "net/http/pprof"

	"github.com/goccy/go-json"
	"github.com/ijimiji/yolist/internal/config"
	"github.com/ijimiji/yolist/internal/parser"
	"golang.org/x/tools/go/packages"
)

func main() {
	var (
		err error
		req packages.DriverRequest
		dr  DriverResponse
	)

	cfg := config.Load()

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

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	dump, _ := json.Marshal(map[string]any{
		"args": os.Args,
		"req":  req,
		"cwd":  cwd,
	})
	os.WriteFile(path.Join("/Users/larynjahor/gits/yolist/log", time.Now().String()), dump, os.ModePerm)

	env := p.Env()

	dr.GoVersion = env.MinorVersion()
	dr.Arch = env.GOARCH
	dr.Compiler = "gc"

	dr.Packages, err = p.Packages()
	if err != nil {
		panic(err)
	}

	for _, p := range dr.Packages {
		if !p.DepOnly {
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
