package driver

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"path"
	"slices"
	"strings"

	"go/parser"
	"go/token"

	"github.com/larynjahor/spd/container"
	"github.com/larynjahor/spd/pkg/env"
	"github.com/larynjahor/spd/pkg/locator"
	"github.com/larynjahor/spd/pkg/tag"
	"github.com/larynjahor/spd/util"

	"golang.org/x/tools/go/packages"
)

func New(
	fs fs.FS,
	locator *locator.Locator,
	evaler *tag.Evaler,
	env *env.Env,
) *Driver {
	return &Driver{
		fs:             fs,
		PackageLocator: locator,
		TagEvaler:      evaler,
		Env:            env,
	}
}

type Driver struct {
	fs             fs.FS
	PackageLocator *locator.Locator
	TagEvaler      *tag.Evaler
	Env            *env.Env
}

func (d *Driver) Do(ctx context.Context, req *packages.DriverRequest) (*packages.DriverResponse, error) {
	root := util.Up(d.Env.GOMOD)
	targets := d.Env.Targets

	state := &State{
		fset:         token.NewFileSet(),
		rootPackages: container.NewSet[string](1024),
		packageCache: make(map[string]*packages.Package, 1024),
	}

	for _, target := range targets {
		err := fs.WalkDir(d.fs, path.Join(root, target), func(fullPath string, entry fs.DirEntry, err error) error {
			if !entry.IsDir() {
				return nil
			}

			// slog.DebugContext(ctx, "parse dir", slog.String("dir", fullPath))

			id, err := d.PackageLocator.GetPackageID(fullPath)
			if err != nil {
				return err
			}

			if id == "" {
				slog.ErrorContext(ctx, "empty package id", slog.String("path", fullPath))

				return nil
			}

			if err := d.parseDirectory(ctx, state, id, fullPath); err != nil {
				slog.ErrorContext(ctx, "parse root directory", "err", err)
				return nil
			}

			state.rootPackages.Add(id)

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk target=%s: %w", target, err)
		}
	}

	builtinDir, err := d.PackageLocator.GetPath("builtin")
	if err != nil {
		return nil, err
	}

	if err := d.parseDirectory(ctx, state, "builtin", builtinDir); err != nil {
		return nil, err
	}

	for k, v := range state.packageCache {
		if v == nil {
			delete(state.packageCache, k)
		}
	}

	rootPackages := make([]string, 0, 1024)
	for id := range state.rootPackages.All() {
		if _, ok := state.packageCache[id]; !ok {
			continue
		}

		if len(state.packageCache[id].CompiledGoFiles) == 0 {
			continue
		}

		rootPackages = append(rootPackages, id)
	}

	resp := &packages.DriverResponse{
		NotHandled: false,
		Compiler:   "gc",
		Arch:       d.Env.GOARCH,
		Roots:      rootPackages,
		Packages:   slices.Collect(maps.Values(state.packageCache)),
		GoVersion:  d.Env.MinorVersion(),
	}

	resp.Roots = append(resp.Roots, "builtin")

	return resp, nil
}

func (d *Driver) parseDirectory(ctx context.Context, state *State, id string, dirPath string) error {
	if _, ok := state.packageCache[id]; ok {
		return nil
	}

	state.packageCache[id] = &packages.Package{
		ID:              id,
		Name:            "",
		PkgPath:         id,
		GoFiles:         []string{},
		CompiledGoFiles: []string{},
		OtherFiles:      []string{},
		EmbedFiles:      []string{},
		EmbedPatterns:   []string{},
		IgnoredFiles:    []string{},
		Imports:         map[string]*packages.Package{},
	}

	state.packageCache[id+"_test"] = &packages.Package{
		ID:              id + "_test",
		Name:            "",
		PkgPath:         id,
		GoFiles:         []string{},
		CompiledGoFiles: []string{},
		OtherFiles:      []string{},
		EmbedFiles:      []string{},
		EmbedPatterns:   []string{},
		IgnoredFiles:    []string{},
		Imports:         map[string]*packages.Package{},
	}

	es, err := fs.ReadDir(d.fs, dirPath)
	if err != nil {
		return err
	}

	for _, e := range es {
		if e.IsDir() {
			continue
		}

		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}

		if err := d.parseFile(ctx, state, id, path.Join(dirPath, e.Name())); err != nil {
			return err
		}
	}

	for importID, _ := range state.packageCache[id].Imports {
		if _, ok := state.packageCache[importID]; ok {
			continue
		}

		packagePath, err := d.PackageLocator.GetPath(importID)
		if err != nil {
			slog.ErrorContext(ctx, "find package", "package", importID)
			continue
		}

		if err := d.parseDirectory(ctx, state, importID, packagePath); err != nil {
			slog.ErrorContext(ctx, "parse additional directory", "directory", packagePath)
			continue
		}
	}

	if len(state.packageCache[id].CompiledGoFiles) == 0 {
		state.packageCache[id] = nil
	}

	if len(state.packageCache[id+"_test"].CompiledGoFiles) == 0 {
		state.packageCache[id+"_test"] = nil
	}

	return nil
}

func (d *Driver) parseFile(ctx context.Context, state *State, id string, filePath string) error {
	f, err := d.fs.Open(filePath)
	if err != nil {
		return err
	}

	defer f.Close()

	tokens := strings.Split(filePath, "/")

	astFile, err := parser.ParseFile(state.fset, tokens[len(tokens)-1], f, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return err
	}

	packageName := astFile.Name.String()
	testOnly := strings.HasSuffix(packageName, "_test")

	if testOnly {
		id = id + "_test"
	}

	for _, g := range astFile.Comments {
		if g == nil {
			continue
		}

		for _, c := range g.List {
			if c == nil {
				continue
			}

			if !strings.HasPrefix(c.Text, "//go:build") {
				continue
			}

			if !d.TagEvaler.Eval(strings.TrimPrefix(c.Text, "//go:build"), d.Env.Tags) {
				state.packageCache[id].IgnoredFiles = append(state.packageCache[id].IgnoredFiles, "/"+filePath)
				return nil
			}
		}
	}

	for _, imp := range astFile.Imports {
		importID := strings.Trim(imp.Path.Value, "\"")
		state.packageCache[id].Imports[importID] = &packages.Package{
			ID: importID,
		}
	}

	state.packageCache[id].Name = packageName
	state.packageCache[id].GoFiles = append(state.packageCache[id].GoFiles, "/"+filePath)
	state.packageCache[id].CompiledGoFiles = append(state.packageCache[id].CompiledGoFiles, "/"+filePath)

	return nil
}

type State struct {
	fset         *token.FileSet
	rootPackages *container.Set[string]
	packageCache map[string]*packages.Package
}
