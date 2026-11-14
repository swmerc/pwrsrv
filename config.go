package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LocalServer LocalServerConfig `yaml:"server"`
	ProSwitch   ProSwitchConfig   `yaml:"proswitch"`
}

type ProSwitchConfig struct {
	Url        string `yaml:"url"`
	User       string `yaml:"user"`
	Password   string `yaml:"password"`
	IgnoreCert bool   `yaml:"ignore_cert"`
}

type LocalServerConfig struct {
	Port uint `yaml:"port"`
}

func getConfig(path string) (Config, error) {
	var err error = nil
	config := Config{}

	// Defaults
	config.LocalServer.Port = 8000

	// Load the file over defaults
	if len(path) == 0 {
		err = fmt.Errorf("path to config file not supplied")
	} else {
		var f *os.File = nil

		if f, err = os.Open(path); err == nil {
			err = yaml.NewDecoder(f).Decode(&config)
			f.Close()
		}
	}

	// Save some potential pain later
	if err == nil {
		if len(config.ProSwitch.Url) == 0 {
			err = fmt.Errorf("%s: empty server URL", path)
		} else if len(config.ProSwitch.User) == 0 {
			err = fmt.Errorf("%s: empty user", path)
		} else if len(config.ProSwitch.User) == 0 {
			err = fmt.Errorf("%s: empty password", path)
		}
	}

	return config, err
}
