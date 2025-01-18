package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/exp/maps"
	"golang.org/x/mod/modfile"
)

const (
	numWorkers = 16
)

func unused() {
	log.Println()
}

var tagRE = regexp.MustCompile(".*go:build ([a-zA-Z0-9]+)")

var (
	ErrPackageNotFound = errors.New("package not found")
	ErrDirNotResolved  = errors.New("cannot resolve directory")
	ErrIgnore          = errors.New("ignore")
)

func New() (*Parser, error) {
	env, err := parseGoEnv()
	if err != nil {
		return nil, err
	}

	env.GOVENDOR = true // TODO

	return &Parser{
		Env:          env,
		cache:        make(map[string]*Package, 1024),
		knownModules: make(map[string]Module, 16),
		knownDirs:    make(map[string]struct{}, 1024),
	}, nil
}

type Parser struct {
	Env          Env
	mtx          sync.Mutex
	path         []string
	tags         []string
	cache        map[string]*Package // id
	knownModules map[string]Module
	knownDirs    map[string]struct{} // name
}

func (p *Parser) ParseTargets(targets []string) ([]*Package, error) {
	for _, t := range targets {
		moduleDir, err := p.findModuleDir(t)
		if err != nil {
			return nil, err
		}

		p.path = append(p.path, moduleDir)
		if p.Env.GOVENDOR {
			p.path = append(p.path, path.Join(moduleDir, "vendor"))
		}

		m, err := p.parseModule(path.Join(moduleDir, "go.mod"))
		if err != nil {
			return nil, err
		}

		p.knownModules[moduleDir] = *m
	}

	p.path = append(p.path, path.Join(p.Env.GOROOT, "src"))

	if !p.Env.GOVENDOR {
		p.path = append(p.path, p.Env.GOPATH)
	}

	targetsCh := make(chan string)
	var wg sync.WaitGroup

	for range numWorkers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for t := range targetsCh {
				if err := p.parse(t); err != nil {
					panic(err)
				}
			}
		}()
	}

	for _, t := range targets {
		targetsCh <- t
	}

	close(targetsCh)

	wg.Wait()

	if err := p.parseDirectory("builtin", path.Join(p.Env.GOROOT, "src", "builtin")); err != nil {
		return nil, err
	}

	return maps.Values(p.cache), nil
}

func (p *Parser) parse(quasiPackage string) error {
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
		if e.IsDir() {
			err := p.parse(path.Join(quasiPackage, e.Name()))
			if err != nil {
				return err
			}

		}
	}

	return nil
}

// returns package id
func (p *Parser) resolveDirectory(dir string) (string, error) {
	for _, ppath := range p.path {
		suffix := strings.TrimPrefix(dir, ppath)
		if suffix == dir {
			continue
		}

		m, ok := p.knownModules[ppath]
		if ok {
			return path.Join(m.Path, suffix), nil
		}

		return suffix, nil
	}

	return "", fmt.Errorf("%v %s", ErrDirNotResolved, dir)
}

// returns package dir
func (p *Parser) resolvePackage(id string) (string, error) {
	p.mtx.Lock()

	if pkg, ok := p.cache[id]; ok {
		p.mtx.Unlock()
		return pkg.Dir, nil
	}

	p.mtx.Unlock()

	for _, root := range p.path {
		packagePath := path.Join(root, id)
		if info, err := os.Stat(packagePath); err == nil && info.IsDir() {
			return packagePath, nil
		}
	}

	for mDir, m := range p.knownModules {
		packagePath := path.Join(mDir, strings.TrimPrefix(id, m.Path))
		if info, err := os.Stat(packagePath); err == nil && info.IsDir() {
			return packagePath, nil
		}
	}

	return "", ErrPackageNotFound
}

type astPackage struct {
	Files map[string]ast.File
}

func (p *Parser) parseDirectory(id string, dir string) error {
	p.mtx.Lock()

	if _, ok := p.knownDirs[dir]; ok {
		p.mtx.Unlock()
		return nil
	}

	p.knownDirs[dir] = struct{}{}

	p.mtx.Unlock()

	es, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	pkgs := map[string]astPackage{}

	for _, e := range es {
		if e.IsDir() || !isGoFile(e.Name()) {
			continue
		}

		fset := token.NewFileSet()
		fileName := path.Join(dir, e.Name())

		f, err := parser.ParseFile(fset, fileName, nil, parser.ImportsOnly|parser.ParseComments)
		switch {
		case errors.Is(err, fs.ErrNotExist) || f == nil:
			continue
		case err != nil:
			return err
		}

		var ignored bool
		for _, g := range f.Comments {
			if g == nil {
				continue
			}

			for _, c := range g.List {
				if c == nil {
					continue
				}

				tokens := tagRE.FindStringSubmatch(c.Text)
				if len(tokens) > 1 && !slices.Contains(p.tags, tokens[1]) {
					ignored = true
				}
			}
		}

		if ignored {
			continue
		}

		packageName := f.Name.Name

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

		isTestPackage := strings.HasSuffix(name, "_test")

		if isTestPackage {
			finalID = fmt.Sprintf("%s_test [%s.test]", finalID, finalID)
			packagePath += "_test"
		}

		flatImports := make(map[string]string)

		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, "\"")

				flatImports[path] = path

				packageDir, err := p.resolvePackage(path)

				switch {
				case errors.Is(err, ErrPackageNotFound):
					log.Printf("%s not found\n", path)
					continue
				case err != nil:
					return err
				}

				if err := p.parseDirectory(path, packageDir); err != nil {
					return err
				}
			}
		}

		p.mtx.Lock()

		p.cache[finalID] = &Package{
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

		p.mtx.Unlock()
	}

	return nil
}

func (p *Parser) findModuleDir(dir string) (string, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, e := range es {
		if e.Name() == "go.mod" {
			return dir, nil
		}
	}

	up, err := upDir(dir)
	if err != nil {
		return "", err
	}

	return p.findModuleDir(up)
}

func (p *Parser) parseModule(goModPath string) (*Module, error) {
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
		Path: mod.Module.Mod.Path,
		Dir:  dir,
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

func Must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}

	return val
}

type Env struct {
	GOVENDOR  bool
	GOMOD     string
	GOPATH    string
	GOROOT    string
	GOARCH    string
	GOVERSION string
}

func (e *Env) MinorVersion() int {
	tokens := strings.Split(e.GOVERSION, ".")
	switch len(tokens) {
	case 2, 3:
		return Must(strconv.Atoi(tokens[1]))
	default:
		panic("wrong version format")
	}
}

func parseGoEnv() (zero Env, _ error) {
	marshaled, err := exec.Command("go", "env", "-json").Output()
	if err != nil {
		return zero, err
	}

	if err := json.Unmarshal(marshaled, &zero); err != nil {
		return zero, err
	}

	return zero, nil
}

func isGoFile(name string) bool {
	return strings.HasSuffix(name, ".go")
}
