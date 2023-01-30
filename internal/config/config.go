package config

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	Hass      HassConfig
	People    map[string]PersonConfig
	Locations map[string]LocationConfig
}

type HassConfig struct {
	Server string `yaml:"server"`
	Token  string `yaml:"token"`
}

type PersonConfig struct {
	Name               string `yaml:"name"`
	NotificationDevice string `yaml:"notification_device"`
}

type LocationConfig struct {
	Name      string `yaml:"name"`
	Owner     string `yaml:"owner"`
	OwnerName string `yaml:"owner_name"`
}

func LoadConfig(fileName string) (*Config, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	output := Config{}

	decoder := yaml.NewDecoder(file)
	decoder.Decode(&output)

	return &output, nil
}
