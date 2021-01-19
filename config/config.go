package config

import (
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v3"
)

// YamlConfig ...
type YamlConfig struct {
	DatabaseConfig `yaml:"Database"`
	RedisConfig    `yaml:"Redis"`
	HTTPConfig     `yaml:"Http"`
}

// DatabaseConfig ...
type DatabaseConfig struct {
	Host     string `yaml:"Host"`
	Port     int    `yaml:"Port"`
	UserName string `yaml:"UserName"`
	Password string `yaml:"Password"`
	DBName   string `yaml:"DBName"`
	TimeZone string `yaml:"TimeZone"`
}

// RedisConfig ...
type RedisConfig struct {
	Host     string `yaml:"Host"`
	Port     int    `yaml:"Port"`
	Password string `yaml:"Password"`
}

// HTTPConfig ...
type HTTPConfig struct {
	Port         int           `yaml:"Port"`
	ReadTimeout  time.Duration `yaml:"ReadTimeout"`
	WriteTimeout time.Duration `yaml:"WriteTimeout"`
}

// ReadConfig ...
func ReadConfig(configPath string) (*YamlConfig, error) {
	var yc YamlConfig
	yamlFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(yamlFile, &yc)
	if err != nil {
		return nil, err
	}

	return &yc, nil
}
