package utils

import (
	"github.com/jinzhu/configor"
)

type RuleConfig struct {
	Name          string   `yaml:"name"`
	Topic         string   `yaml:"topic"`
	Topics        []string `yaml:"topics"`
	ExcludeTopics []string `yaml:"excludeTopics"`
	Concurrency   int      `yaml:"concurrency"`
	Filter        string   `yaml:"filter"`
	TaskType      string   `yaml:"taskType"`
}

var Config = struct {
	Kafka struct {
		BrokerList string `yaml:"brokerList"`
	} `yaml:"kafka"`

	Options struct {
		GroupIDPrefix                string `yaml:"groupIDPrefix"`
		MetadataWatchGroupID         string `yaml:"metadataWatchGroupID"`
		MetadataWatchRefreshInterval int64  `yaml:"metadataWatchRefreshInterval"`
	} `yaml:"options"`

	JobRunner struct {
		Endpoint      string   `yaml:"endpoint"`
		ExcludeFields []string `yaml:"excludeFields"`
	} `yaml:"jobRunner"`

	Rules []RuleConfig `yaml:"rules"`
}{}

func InitConfig() error {
	return configor.Load(&Config, "conf/config.yml")
}
