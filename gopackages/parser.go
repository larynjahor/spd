package gopackages

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/mod/modfile"
)

var (
	ErrPackageNotFound = errors.New("package not found")
	ErrDirNotResolved  = errors.New("cannot resolve directory")
	ErrIgnore          = errors.New("ignore")
)

func NewParser(env Env) (*Parser, error) {
	root, err := upDir(env.GOMOD)
	if err != nil {
		return nil, err
	}

	var targets []string

	for _, relative := range env.Targets {
		abs := path.Join(root, relative)

		s, err := os.Stat(abs)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		case err != nil:
			return nil, err
		}

		if !s.IsDir() {
			continue
		}

		targets = append(targets, abs)
	}

	modules := map[string]Module{}

	// we might want to expand this logic to gowork
	for _, m := range []string{env.GOMOD, path.Join(env.GOROOT, "src", "go.mod")} {
		mod, err := parseModule(m)
		if err != nil {
			return nil, err
		}

		modules[mod.Dir] = *mod
	}

	return &Parser{
		root:         root,
		targets:      targets,
		env:          env,
		cache:        make(map[string]*Package, 8192),
		knownDirs:    make(map[string]struct{}, 8192),
		tags:         env.Tags,
		knownModules: modules,
	}, nil
}

type Parser struct {
	env          Env
	root         string
	tags         []string
	cache        map[string]*Package // id
	targets      []string            // path
	knownModules map[string]Module
	knownDirs    map[string]struct{} // path
}

func (p *Parser) Env() Env {
	return p.env
}

func (p *Parser) Packages(patterns []string) ([]*Package, error) {
	for _, pattern := range patterns {
		switch {
		case strings.HasPrefix(pattern, "file="):
			file := strings.TrimPrefix(pattern, "file=")
			dir, _ := path.Split(file)

			if err := p.parse(dir, 1); err != nil {
				slog.Error("parse package", slog.Any("err", err), slog.String("pattern", pattern), slog.String("res_pattern", dir))
				continue
			}
		case strings.HasSuffix(pattern, "/..."):
			packagePath := p.root
			patternPath := strings.TrimSuffix(strings.TrimSuffix(pattern, "..."), string(os.PathSeparator))

			if patternPath != "" {
				packagePath = path.Join(packagePath, patternPath)
			}

			if err := p.parse(packagePath, -1); err != nil {
				slog.Error("parse package", slog.Any("err", err), slog.String("pattern", pattern), slog.String("res_pattern", packagePath))
				continue
			}

		default:
			dir, err := p.resolvePackage(pattern)
			if err != nil {
				slog.Warn("cannot locate package", slog.String("pattern", pattern), slog.Any("err", err))
				continue
			}

			if err := p.parseDirectory(pattern, dir); err != nil {
				slog.Error("cannot parse package", slog.String("pattern", pattern), slog.String("directory", dir), slog.Any("err", err))
				continue
			}
		}
	}

	return maps.Values(p.cache), nil
}

func (p *Parser) parse(quasiPackage string, depth int) error {
	if depth == 0 {
		return nil
	}

	id, err := p.resolveDirectory(quasiPackage)
	if err != nil {
		return err
	}

	if err := p.parseDirectory(id, quasiPackage); err != nil {
		return err
	}

	es, err := os.ReadDir(quasiPackage)
	if err != nil {
		return err
	}

	for _, e := range es {
		if !e.IsDir() {
			continue
		}

		dir := path.Join(quasiPackage, e.Name())

		if !p.allowedByTargets(dir) {
			continue
		}

		if err := p.parse(dir, depth-1); err != nil {
			return err
		}
	}

	return nil
}

// returns package id
func (p *Parser) resolveDirectory(dir string) (string, error) {
	for _, mod := range p.knownModules {
		id, ok := mod.LocateDir(dir)
		if ok {
			return id, nil
		}
	}

	rest := strings.TrimPrefix(dir, path.Join(p.env.GOPATH, "pkg", "mod"))
	if rest != dir {
		return strings.TrimPrefix(rest, "/"), nil
	}

	return "", ErrDirNotResolved
}

// returns package dir
func (p *Parser) resolvePackage(id string) (string, error) {
	if pkg, ok := p.cache[id]; ok {
		return pkg.Dir, nil
	}

	for _, m := range p.knownModules {
		p, ok := m.LocateID(id)
		if ok {
			return p, nil
		}
	}

	path := path.Join(p.env.GOPATH, "pkg", "mod", id)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path, nil
	}

	return "", ErrPackageNotFound
}

type astPackage struct {
	Files map[string]ast.File
}

func (p *Parser) parseDirectory(id string, dir string) error {
	if _, ok := p.knownDirs[dir]; ok {
		return nil
	}

	p.knownDirs[dir] = struct{}{}

	if strings.HasPrefix(dir, p.env.GOROOT) && (strings.Contains(dir, "testdata") || strings.Contains(dir, "internal")) {
		return nil
	}

	es, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	pkgs := map[string]astPackage{}

	for _, e := range es {
		if e.IsDir() || !isGoFile(e.Name()) {
			continue
		}

		fileName := path.Join(dir, e.Name())

		if strings.HasPrefix(fileName, "_test.go") {
			continue // TODO
		}

		if _, ok := pkgs[packageName]; !ok {
			pkgs[packageName] = astPackage{
				Files: map[string]ast.File{},
			}
		}

		pkgs[packageName].Files[fileName] = *f
	}

	for name, pkg := range pkgs {
		finalID := id
		packagePath := id

		flatImports := make(map[string]string)

		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, "\"")

				flatImports[path] = path

				if path == "C" {
					continue
				}

				packageDir, err := p.resolvePackage(path)

				switch {
				case errors.Is(err, ErrPackageNotFound):
					slog.Warn("package not found", slog.String("path", path))
					continue
				case err != nil:
					return err
				}

				if err := p.parseDirectory(path, packageDir); err != nil {
					return err
				}
			}
		}

		p.cache[finalID] = &Package{
			DepOnly:         !p.isRootPackage(dir),
			ID:              finalID,
			Name:            name,
			PkgPath:         packagePath,
			Dir:             dir,
			Errors:          nil,
			GoFiles:         maps.Keys(pkg.Files),
			CompiledGoFiles: maps.Keys(pkg.Files),
			OtherFiles:      []string{},
			EmbedFiles:      []string{},
			EmbedPatterns:   []string{},
			IgnoredFiles:    []string{},
			ExportFile:      "",
			Target:          "",
			Imports:         flatImports,
		}
	}

	return nil
}

func (p *Parser) parseFile(id string, fileName string) (*Package, error) {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, fileName, nil, parser.ImportsOnly|parser.ParseComments)
	switch {
	case errors.Is(err, fs.ErrNotExist) || f == nil:
		return nil, nil
	case err != nil:
		return nil, err
	}

	for _, g := range f.Comments {
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

			if !p.allowedByTags(strings.TrimPrefix(c.Text, "//go:build")) {
				return nil, nil
			}
		}
	}

	packageName := f.Name.Name

	isTestPackage := strings.HasSuffix(packageName, "_test")

	if isTestPackage {
		return nil, nil

		// finalID = fmt.Sprintf("%s_test [%s.test]", finalID, finalID)
		// packagePath += "_test"
	}

	return nil, nil
}

func (p *Parser) allowedByTargets(dir string) bool {
	for _, t := range p.targets {
		if strings.HasPrefix(dir, t) {
			return true
		}
	}

	return len(p.targets) == 0
}

func (p *Parser) allowedByTags(s string) bool {
	logger := slog.With(slog.String("build directive", s))

	out := newStack[string]()
	ops := newStack[string]()

	for _, token := range strings.Fields(s) {
		temp := []string{}

		tempString := ""

		for i, ch := range token {
			switch ch {
			case '!', '(', ')':
				if tempString != "" {
					temp = append(temp, tempString)
					tempString = ""
				}

				temp = append(temp, string(ch))
			default:
				tempString += string(ch)
				if i == len(token)-1 {
					temp = append(temp, tempString)
				}
			}
		}

		for _, t := range temp {
			switch t {
			case "!":
				ops.Push(t)
			case "&&":
				for !ops.Empty() && !(ops.Top() == "&&" || ops.Top() == "||" || ops.Top() == "(") {
					out.Push(ops.Pop())
				}

				ops.Push(t)
			case "||":
				for !ops.Empty() && !(ops.Top() == "||" || ops.Top() == "(") {
					out.Push(ops.Pop())
				}

				ops.Push(t)
			case "(":
				ops.Push("(")
			case ")":
				for !ops.Empty() {
					cur := ops.Pop()
					if cur == "(" {
						break
					}

					out.Push(cur)
				}
			default:
				out.Push(t)
			}
		}

	}

	for !ops.Empty() {
		out.Push(ops.Pop())
	}

	eval := newStack[bool]()

	for _, t := range out.Values() {
		switch t {
		case "!":
			if eval.Empty() {
				logger.Error("no operand for !")
				return false
			}

			eval.Push(!eval.Pop())
		case "||", "&&":
			if eval.Empty() {
				logger.Error("no operand for || or &&")
				return false
			}

			first := eval.Pop()

			if eval.Empty() {
				logger.Error("no operand for || or &&")
				return false
			}

			second := eval.Pop()

			if t == "&&" {
				eval.Push(first && second)
			} else {
				eval.Push(first || second)
			}
		default:
			eval.Push(slices.Contains(p.env.Tags, t))
		}
	}

	if eval.Empty() {
		logger.Error("extra tokens in result stack")
		return false
	}

	return eval.Pop()
}

func (p *Parser) isRootPackage(pkgDir string) bool {
	return p.allowedByTargets(pkgDir)
}

func parseModule(goModPath string) (*Module, error) {
	f, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}

	mod, err := modfile.Parse(goModPath, f, nil)
	if err != nil {
		return nil, err
	}

	defer mod.Cleanup()

	dir, _ := path.Split(goModPath)
	dir = strings.TrimSuffix(dir, string(os.PathSeparator))

	m := &Module{
		Path:      mod.Module.Mod.Path,
		Dir:       dir,
		VendorDir: path.Join(dir, "vendor"),
	}

	return m, nil
}

func upDir(dir string) (string, error) {
	parts := strings.Split(dir, string(os.PathSeparator))

	if len(parts) <= 1 {
		return "", errors.New("not found")
	}

	return path.Join(append([]string{string(os.PathSeparator)}, parts[:len(parts)-1]...)...), nil
}

func isGoFile(name string) bool {
	return strings.HasSuffix(name, ".go")
}
