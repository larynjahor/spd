package main

import (
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/goccy/go-json"
	"github.com/larynjahor/spd/gopackages"
	"github.com/larynjahor/spd/xslog"
	"golang.org/x/tools/go/packages"
)

func main() {
	c := xslog.Auto()
	defer c.Close()

	if err := run(); err != nil {
		log.Fatalln(err)
	}
}

func run() error {
	var (
		started = time.Now()
		req     packages.DriverRequest
		dr      DriverResponse
	)

	slog.Info("started spd", slog.String("args", strings.Join(os.Args, " ")))

	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		return err
	}

	env, err := gopackages.ParseEnv(req.Env)
	if err != nil {
		return err
	}

	parser, err := gopackages.NewParser(env)
	if err != nil {
		return err
	}

	dr.GoVersion = env.MinorVersion()
	dr.Arch = env.GOARCH
	dr.Compiler = "gc"

	dr.Packages, err = parser.Packages(os.Args[1:])
	if err != nil {
		slog.Error("failed to get packages", slog.Any("err", err), slog.String("goroot", env.GOROOT), slog.String("gomod", env.GOMOD))
		return err
	}

	for _, p := range dr.Packages {
		if !p.DepOnly {
			dr.Roots = append(dr.Roots, p.ID)
		}
	}

	dr.Roots = append(dr.Roots, "builtin")

	slog.Info("exiting spd", slog.Int("numPackages", len(dr.Packages)), slog.Duration("took", time.Since(started)))

	return writeResponse(&dr)
}

func writeResponse(dr *DriverResponse) error {
	if err := json.NewEncoder(os.Stdout).Encode(dr); err != nil {
		return err
	}

	return nil
}

type DriverResponse struct {
	NotHandled bool

	Compiler string
	Arch     string

	Roots []string `json:",omitempty"`

	Packages []*gopackages.Package

	GoVersion int
}
