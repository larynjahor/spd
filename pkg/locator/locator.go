package locator

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"strings"

	"github.com/larynjahor/spd/pkg"
	"github.com/larynjahor/spd/pkg/env"
	"github.com/larynjahor/spd/util"
	"golang.org/x/mod/modfile"
	"golang.org/x/sync/errgroup"
)

type Locator struct {
	modules map[string]string
	indices map[string]*node
}

func NewLocator(
	pkgFS fs.FS,
	env *env.Env,
) (*Locator, error) {
	modFile, err := pkgFS.Open(env.GOMOD)
	if err != nil {
		return nil, err
	}
	defer modFile.Close()

	modContent, err := io.ReadAll(modFile)
	if err != nil {
		return nil, err
	}

	modName, err := moduleName(env.GOMOD, modContent)
	if err != nil {
		return nil, err
	}

	slog.Debug("got modname", slog.String("name", modName))

	indices := map[string]*node{}

	roots := []string{
		path.Join(env.GOROOT, "src"),
		path.Join(env.GOROOT, "src/vendor"),
		path.Join(util.Up(env.GOMOD), "vendor"),
		path.Join(env.GOPATH, "pkg", "mod"),
	}

	var eg errgroup.Group

	for _, root := range roots {
		indices[root] = &node{
			Children: map[string]*node{},
		}

		eg.Go(func() error {
			deep := root == path.Join(env.GOROOT, "src")
			return fs.WalkDir(pkgFS, root, func(fullPath string, d fs.DirEntry, err error) error {
				if fullPath == root {
					return nil
				}

				if len(strings.Split(strings.TrimPrefix(fullPath, root), "/")) > 2 && !deep {
					return fs.SkipDir
				}

				switch d.Name() {
				case "vendor":
					return fs.SkipDir
				case "go.mod":
					if !deep {
						return fs.SkipDir
					}
				}

				if !d.IsDir() {
					return nil
				}

				packageID := strings.TrimPrefix(fullPath, root+"/")
				packageTokens := strings.Split(packageID, "/")

				indices[root].Add(packageTokens)

				return nil
			})
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("wait for index routines")
	}

	slog.Debug("initialized locator")

	// dump, _ := json.MarshalIndent(indices, "", " ")
	// fmt.Println(string(dump))

	return &Locator{
		indices: indices,
		modules: map[string]string{
			modName: util.Up(env.GOMOD),
		},
	}, nil
}

func (l *Locator) GetPackageID(dirPath string) (string, error) {
	for prefix, path := range l.modules {
		if strings.HasPrefix(dirPath, path) {
			return prefix + strings.TrimPrefix(dirPath, path), nil
		}
	}

	return "", pkg.ErrUnimplemented
}

func (l *Locator) GetPath(id string) (string, error) {
	for prefix, rootPath := range l.modules {
		if !strings.HasPrefix(id, prefix) {
			continue
		}

		return path.Join(rootPath, strings.TrimPrefix(id, prefix)), nil
	}

	for root, node := range l.indices {
		packageTokens := strings.Split(id, "/")
		if !node.Contains(packageTokens) {
			continue
		}

		return path.Join(append([]string{root}, packageTokens...)...), nil
	}

	return "", pkg.ErrPackageNotFound
}

func moduleName(goModPath string, content []byte) (string, error) {
	mod, err := modfile.Parse(goModPath, content, nil)
	if err != nil {
		return "", err
	}

	defer mod.Cleanup()

	return mod.Module.Mod.Path, nil
}

type node struct {
	Children map[string]*node
}

func (n *node) Contains(keys []string) bool {
	if len(keys) == 0 {
		return false
	}

	for key, c := range n.Children {
		if key != keys[0] {
			continue
		}

		if len(c.Children) == 0 || len(keys) == 1 {
			return true
		}

		return c.Contains(keys[1:])
	}

	return false
}

func (n *node) Add(keys []string) {
	if len(keys) == 0 {
		return
	}

	var found *node

	for key, child := range n.Children {
		if key == keys[0] {
			found = child
		}
	}

	if found == nil {
		found = &node{
			Children: map[string]*node{},
		}

		n.Children[keys[0]] = found
	}

	found.Add(keys[1:])
}
