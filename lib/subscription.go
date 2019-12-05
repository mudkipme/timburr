package lib

import (
	"github.com/mudkipme/timburr/utils"
)

type SubscriptionConfig struct {
	BrokerList    string
	GroupIDPrefix string
}

type Subscription interface {
	Subscribe() error
	Unsubscribe()
}

func NewSubscription(config *SubscriptionConfig, rule utils.RuleConfig) Subscription {
	var s Subscription
	if isBasicRule(rule) {
		s = &BasicSubscription{
			rule:   rule,
			config: config,
		}
	} else {
		s = &RegexSubscription{
			rule:   rule,
			config: config,
		}
	}
	return s
}
