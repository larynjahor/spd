package gopackages

import (
	"encoding/json"
	"fmt"
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

	for _, v := range vars {
		tokens := strings.SplitN(v, "=", 2)
		if len(tokens) < 2 {
			return zero, fmt.Errorf("invalid env var [%s]", v)
		}

		k, v := tokens[0], tokens[1]

		switch k {
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
		}
	}

	return zero, nil
}

type Env struct {
	Targets []string `json:"-"`
	Vendor  bool     `json:"-"`

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
