package data

import (
	"embed"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing/fstest"
	"time"
)

//go:embed root/*
var testFS embed.FS

var FS = make(fstest.MapFS)

func init() {
	err := fs.WalkDir(testFS, "root", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		f, err := testFS.Open(path)
		if err != nil {
			panic(err)
		}

		defer f.Close()

		bytes, err := io.ReadAll(f)
		if err != nil {
			panic(err)
		}

		newPath := strings.TrimSuffix(path, ".test")

		FS[newPath] = &fstest.MapFile{
			Data:    bytes,
			Mode:    os.ModePerm,
			ModTime: time.Now(),
			Sys:     nil,
		}

		return nil
	})
	if err != nil {
		panic(err)
	}
}
