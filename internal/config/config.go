package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Hass      HassConfig     `yaml:"hass"`
	People    []PersonConfig `yaml:"people"`
	Templates struct {
		Arrive struct {
			You   string `yaml:"you"`
			Other string `yaml:"other"`
		} `yaml:"arrive"`
		Leave struct {
			You   string `yaml:"you"`
			Other string `yaml:"other"`
		} `yaml:"leave"`
	} `yaml:"templates"`
	Locations []LocationConfig `yaml:"locations"`
}

func (config *Config) FindPerson(id string) (*PersonConfig, bool) {
	for _, person := range config.People {
		if person.ID == id {
			return &person, true
		}
	}

	return nil, false
}

func (config *Config) FindLocation(id string) (*LocationConfig, bool) {
	for _, location := range config.Locations {
		if location.ID == id {
			return &location, true
		}
	}

	return nil, false
}

type HassConfig struct {
	Server string `yaml:"server"`
	Token  string `yaml:"token"`
}

type PersonConfig struct {
	ID                 string `yaml:"id"`
	Name               string `yaml:"name"`
	NotificationDevice string `yaml:"notification_device"`
}

type LocationConfig struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Texts struct {
		Arrive string `yaml:"arrive"`
		Leave  string `yaml:"leave"`
	} `yaml:"texts"`
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
