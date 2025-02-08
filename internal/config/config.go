package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Load() (ret Config) {
	marshaled, err := os.ReadFile(os.ExpandEnv("$HOME/.config/yolist/config.yaml"))
	if err != nil {
		panic(err)
	}

	if err := yaml.Unmarshal(marshaled, &ret); err != nil {
		panic(err)
	}

	return ret
}

type Config struct {
	Patterns map[string][]string `yaml:"patterns"`
}
