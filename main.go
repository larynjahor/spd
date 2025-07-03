package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	_ "net/http/pprof"

	"github.com/goccy/go-json"
	"github.com/larynjahor/spd/logging"
	"github.com/larynjahor/spd/pkg/driver"
	"github.com/larynjahor/spd/pkg/env"
	"github.com/larynjahor/spd/pkg/locator"
	"github.com/larynjahor/spd/pkg/tag"
	"golang.org/x/tools/go/packages"
)

func main() {
	c := logging.Auto()
	defer c.Close()

	ctx := context.Background()

	if err := run(ctx); err != nil {
		slog.ErrorContext(ctx, "run driver", slog.Any("err", err))
	}
}

func run(ctx context.Context) error {
	var (
		started = time.Now()
		req     packages.DriverRequest
	)

	slog.Info("started spd", slog.String("args", strings.Join(os.Args, " ")))

	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		return err
	}

	fs := os.DirFS("/")
	envParser := env.New()
	env, err := envParser.Parse(req.Env)
	if err != nil {
		return fmt.Errorf("parse env: %w", err)
	}

	pkgLocator, err := locator.NewLocator(fs, &env)
	if err != nil {
		return fmt.Errorf("init locator: %w", err)
	}

	tags := tag.New()

	driver := driver.New(fs, pkgLocator, tags, &env)

	resp, err := driver.Do(context.Background(), &req)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		return err
	}

	slog.Info("exiting spd", slog.Int("numPackages", len(resp.Packages)), slog.Duration("took", time.Since(started)))

	return nil
}
