package gopackages

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

func ParseEnv(vars []string) (zero Env, _ error) {
	cwd, err := os.Getwd()
	if err != nil {
		return zero, err
	}

	marshaled, err := exec.Command("go", "env", "-json").Output()
	if err != nil {
		return zero, err
	}

	if err := json.Unmarshal(marshaled, &zero); err != nil {
		return zero, err
	}

	slog.Info(
		"parsed go toolchain environment",
		slog.String("os", zero.GOOS),
		slog.String("arch", zero.GOARCH),
		slog.String("gomod", zero.GOMOD),
		slog.String("goroot", zero.GOROOT),
		slog.String("gopath", zero.GOPATH),
		slog.String("goversion", zero.GOVERSION),
	)

	zero.Tags = append(zero.Tags, zero.GOARCH)

	switch zero.GOOS {
	case "android":
		zero.Tags = append(zero.Tags, "linux", "unix")
	case "ios":
		zero.Tags = append(zero.Tags, "darwin", "unix")
	case "illumos":
		zero.Tags = append(zero.Tags, "solaris", "unix")
	case "linux", "darwin", "bsd", "solaris", "dragonfly", "openbsd", "freebsd", "hurd", "netbsd", "plan9":
		zero.Tags = append(zero.Tags, "unix")
	default:
		zero.Tags = append(zero.Tags, zero.GOOS)
	}

	for _, v := range vars {
		tokens := strings.SplitN(v, "=", 2)
		if len(tokens) < 2 {
			return zero, fmt.Errorf("invalid env var [%s]", v)
		}

		k, v := tokens[0], tokens[1]

		switch k {
		case "CGO_ENABLED":
			if v == "1" {
				zero.Tags = append(zero.Tags, "cgo")
			}
		case "GOFLAGS":
			flags := strings.Split(v, " ")
			for _, flag := range flags {
				if flag == "-mod=vendor" {
					zero.Vendor = true
				}
			}
		case "SPDTARGETS":
			relativeTargets := strings.Split(v, ",")

			for _, t := range relativeTargets {
				if !path.IsAbs(t) {
					t = path.Join(cwd, t)
				}

				zero.Targets = append(zero.Targets, t)
			}
		default:
			slog.Info("got env", slog.String("key", k), slog.String("value", v))
		}
	}

	return zero, nil
}

type Env struct {
	Targets []string `json:"-"`
	Vendor  bool     `json:"-"`
	Tags    []string

	GOMOD     string
	GOPATH    string
	GOROOT    string
	GOARCH    string
	GOVERSION string
	GOOS      string
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
