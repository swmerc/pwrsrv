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
	Address string `yaml:"address"`
	Port    uint   `yaml:"port"`
}

func getConfig(path string) (Config, error) {
	var err error = nil
	config := Config{}

	// Defaults
	config.LocalServer.Address = "127.0.0.1"
	config.LocalServer.Port = 8000

	// Load the file over defaults
	if len(path) == 0 {
		err = fmt.Errorf("path to config file not supplied")
	} else {
		var f *os.File = nil

		if f, err = os.Open(path); err == nil {
			// Reject unknown keys so typos and old key names don't silently
			// leave settings at their defaults
			decoder := yaml.NewDecoder(f)
			decoder.KnownFields(true)
			if err = decoder.Decode(&config); err != nil {
				err = fmt.Errorf("%s: %w", path, err)
			}
			f.Close()
		}
	}

	// Save some potential pain later
	if err == nil {
		if len(config.ProSwitch.Url) == 0 {
			err = fmt.Errorf("%s: proswitch: empty url", path)
		} else if len(config.ProSwitch.User) == 0 {
			err = fmt.Errorf("%s: proswitch: empty user", path)
		} else if len(config.ProSwitch.Password) == 0 {
			err = fmt.Errorf("%s: proswitch: empty password", path)
		}
	}

	return config, err
}
