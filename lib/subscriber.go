package lib

import (
	"sync"
	"time"

	"github.com/mudkipme/timburr/utils"
	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

// Subscriber can subscribe to mutiple rules to consume messages
type Subscriber struct {
	metadataWatcher *MetadataWatcher
	config          *SubScriberConfig
	subscriptions   []Subscription
	mutex           sync.Mutex
}

// SubScriberConfig contains configuration of kafka
type SubScriberConfig struct {
	BrokerList                   string
	GroupIDPrefix                string
	MetadataWatchGroupID         string
	MetadataWatchRefreshInterval time.Duration
}

// DefaultSubscriber creates a subscriber based on config.yml
func DefaultSubscriber() *Subscriber {
	cfg := SubScriberConfig{
		BrokerList:                   utils.Config.Kafka.BrokerList,
		GroupIDPrefix:                utils.Config.Options.GroupIDPrefix,
		MetadataWatchGroupID:         utils.Config.Options.MetadataWatchGroupID,
		MetadataWatchRefreshInterval: time.Millisecond * time.Duration(utils.Config.Options.MetadataWatchRefreshInterval),
	}
	return NewSubscriber(&cfg)
}

// NewSubscriber creates a new subscriber
func NewSubscriber(config *SubScriberConfig) *Subscriber {
	s := &Subscriber{
		config:        config,
		subscriptions: []Subscription{},
	}
	return s
}

// Subscribe creates a new subscription with a rule
func (s *Subscriber) Subscribe(rule utils.RuleConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sub := NewSubscription(&SubscriptionConfig{
		BrokerList:    s.config.BrokerList,
		GroupIDPrefix: s.config.GroupIDPrefix,
	}, rule)
	s.subscriptions = append(s.subscriptions, sub)

	// create a metadata watcher
	if sub, ok := sub.(*RegexSubscription); ok {
		if s.metadataWatcher == nil {
			c, err := kafka.NewConsumer(&kafka.ConfigMap{
				"bootstrap.servers": s.config.BrokerList,
				"group.id":          s.config.MetadataWatchGroupID,
				"auto.offset.reset": "earliest",
			})
			if err != nil {
				return err
			}
			s.metadataWatcher, err = NewMetadataWatcher(c, s.config.MetadataWatchRefreshInterval)
			if err != nil {
				return err
			}
		}
		sub.MetadataWatcher = s.metadataWatcher
	}

	return sub.Subscribe()
}

// Unsubscribe turns off all subscriptions
func (s *Subscriber) Unsubscribe() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, sub := range s.subscriptions {
		sub.Unsubscribe()
	}

	if s.metadataWatcher != nil {
		s.metadataWatcher.Disconnect()
	}
}
