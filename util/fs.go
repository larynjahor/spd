package util

import (
	"os"
	"path"
	"strings"
)

func Up(dir string) string {
	return strings.TrimSuffix(dir, string(os.PathSeparator)+path.Base(dir))
}
