package parser

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/mod/modfile"
)

var tagRE = regexp.MustCompile(".*go:build ([a-zA-Z0-9]+)")

func New() (*Parser, error) {
	env, err := parseGoEnv()
	if err != nil {
		return nil, err
	}

	env.GOVENDOR = true // TODO

	return &Parser{
		Env:          env,
		root:         os.ExpandEnv("$HOME/arcadia"),
		cache:        make(map[string]*Package, 1024),
		knownModules: make(map[string]*Module, 128),
		knownDirs:    make(map[string]struct{}, 1024),
	}, nil
}

type Parser struct {
	Env          Env
	root         string // TODO
	tags         []string
	cache        map[string]*Package // id
	knownModules map[string]*Module  // name
	knownDirs    map[string]struct{} // name
}

func (p *Parser) ParseTargets(targets []string) ([]*Package, error) {
	if err := p.parsePackage("builtin"); err != nil {
		return nil, err
	}

	for _, t := range targets {
		mPath, err := p.findModule(t)
		if err != nil {
			return nil, err
		}

		module, err := p.parseModule(mPath)
		if err != nil {
			return nil, err
		}

		if err := p.parse(module, t); err != nil {
			return nil, err
		}
	}

	return maps.Values(p.cache), nil
}

func (p *Parser) parse(module *Module, dir string) error {
	if err := p.parseDirectory(module, dir); err != nil {
		return err
	}

	es, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, e := range es {
		if e.IsDir() {
			err := p.parse(module, path.Join(dir, e.Name()))
			if err != nil {
				return err
			}

		}
	}

	return nil
}

func (p *Parser) parsePackage(id string) error {
	if _, ok := p.cache[id]; ok {
		return nil
	}

	if !p.Env.GOVENDOR {
		for modPath, m := range p.knownModules {
			if strings.HasPrefix(id, modPath) {
				dir := path.Join(m.Dir, strings.TrimPrefix(id, modPath))

				return p.parseDirectory(m, dir)
			}
		}
	} else {
		vendoredPath := path.Join(p.root, "vendor", id)
		if info, err := os.Stat(vendoredPath); err == nil && info.IsDir() {
			path, err := p.findModule(vendoredPath)
			if err != nil {
				return err
			}

			m, err := p.parseModule(path)
			if err != nil {
				return err
			}

			m.Vendored = true

			return p.parseDirectory(m, vendoredPath)
		}
	}

	stdPath := path.Join(p.Env.GOROOT, "src", id)
	if info, err := os.Stat(stdPath); err == nil && info.IsDir() {
		return p.parseDirectory(&Module{
			Path:      "",
			Version:   "",
			Main:      false,
			Indirect:  false,
			Dir:       p.Env.GOROOT,
			GoMod:     "",
			GoVersion: "",
			Error:     nil,
		}, stdPath)
	} else {
		// log.Println(stdPath)
	}

	rootPath := path.Join(p.Env.GOPATH, id)
	if info, err := os.Stat(rootPath); err == nil && info.IsDir() {
		goModPath, err := p.findModule(rootPath)
		if err != nil {
			return err
		}

		mod, err := p.parseModule(goModPath)
		if err != nil {
			return err
		}

		return p.parseDirectory(mod, rootPath)
	} else {
		// log.Println(rootPath)
	}

	return nil
}

type astPackage struct {
	Files map[string]ast.File
}

func (p *Parser) parseDirectory(module *Module, dir string) error {
	if _, ok := p.knownDirs[dir]; ok {
		return nil
	}

	p.knownDirs[dir] = struct{}{}

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
		flatImports := make(map[string]string)

		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				path := strings.Trim(imp.Path.Value, "\"")

				flatImports[path] = path

				if err := p.parsePackage(path); err != nil {
					return err
				}
			}
		}

		var id string
		switch {
		case module.Dir == p.Env.GOROOT:
			id = strings.TrimPrefix(dir, path.Join(p.Env.GOROOT, "src")+"/")
		default:
			suffix := strings.TrimPrefix(dir, module.Dir)
			suffix = strings.TrimSuffix(suffix, string(os.PathSeparator))
			if len(suffix) == 0 {
				id = module.Path
			} else {
				id = strings.Join([]string{module.Path, suffix}, "/")
			}
		}

		p.cache[id] = &Package{
			ID:              id,
			Name:            name,
			PkgPath:         id,
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
			Module:          nil,
		}
	}

	return nil
}

func (p *Parser) findModule(dir string) (string, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, e := range es {
		if e.Name() == "go.mod" {
			return path.Join(dir, e.Name()), nil
		}
	}

	up, err := upDir(dir)
	if err != nil {
		return "", err
	}

	return p.findModule(up)
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
	goModPath = strings.TrimSuffix(goModPath, string(os.PathSeparator))

	m := &Module{
		Vendored:  false,
		Path:      mod.Module.Mod.Path,
		Version:   mod.Module.Mod.Version,
		Main:      false,
		Indirect:  false,
		Dir:       dir,
		GoMod:     goModPath,
		GoVersion: "1.22.1", // TODO
		Error:     nil,
	}

	p.knownModules[mod.Module.Mod.Path] = m

	return m, nil
}

func upDir(dir string) (string, error) {
	parts := strings.Split(dir, string(os.PathSeparator))

	if len(parts) <= 1 {
		return "", errors.New("not found")
	}

	return path.Join(append([]string{string(os.PathSeparator)}, parts[:len(parts)-1]...)...), nil
}

func Map[I any, O any](xs []I, f func(I) O) []O {
	ret := make([]O, 0, len(xs))

	for _, x := range xs {
		ret = append(ret, f(x))
	}

	return ret
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
