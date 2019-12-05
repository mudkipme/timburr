package lib

import (
	"errors"
	"sync"
	"time"

	"github.com/mudkipme/timburr/utils"
)

type RegexSubscription struct {
	config          *SubscriptionConfig
	rule            utils.RuleConfig
	MetadataWatcher *MetadataWatcher
	ch              chan MetadataWatcherEvent
	stopCh          chan bool
	subscribed      bool
	subscription    *BasicSubscription
	mutex           sync.Mutex
}

func (sub *RegexSubscription) Subscribe() error {
	sub.mutex.Lock()
	defer sub.mutex.Unlock()

	if sub.subscribed {
		return nil
	}
	if sub.MetadataWatcher == nil {
		return errors.New("metadata watcher not exists")
	}
	topics, err := sub.MetadataWatcher.GetTopics()
	if err != nil {
		return err
	}
	err = sub.resubscribe(sub.filteredTopics(topics))
	if err != nil {
		return err
	}

	sub.stopCh = make(chan bool, 1)
	sub.ch = make(chan MetadataWatcherEvent)
	sub.MetadataWatcher.AddListener(sub.ch)
	sub.subscribed = true

	go func() {
		for sub.subscribed {
			select {
			case event := <-sub.ch:
				if event.Err == nil {
					sub.mutex.Lock()
					sub.resubscribe(sub.filteredTopics(event.Topics))
					sub.mutex.Unlock()
				}
			case <-sub.stopCh:
				sub.mutex.Lock()
				sub.MetadataWatcher.RemoveListener(sub.ch)
				sub.ch = nil
				sub.subscribed = false
				sub.mutex.Unlock()
			}
		}
	}()
	return nil
}

func (sub *RegexSubscription) filteredTopics(topics []string) []string {
	return filterTopics(sub.rule, topics)
}

func (sub *RegexSubscription) resubscribe(topics []string) error {
	if sub.subscription != nil {
		sub.subscription.Unsubscribe()
		time.Sleep(time.Second * 5)
	}
	if len(topics) == 0 {
		return nil
	}

	newRule := utils.RuleConfig{
		Name:        sub.rule.Name,
		Topics:      topics,
		Concurrency: sub.rule.Concurrency,
		Filter:      sub.rule.Filter,
		TaskType:    sub.rule.TaskType,
	}
	sub.subscription = &BasicSubscription{
		config: sub.config,
		rule:   newRule,
	}
	return sub.subscription.Subscribe()
}

func (sub *RegexSubscription) Unsubscribe() {
	sub.mutex.Lock()
	defer sub.mutex.Unlock()

	if !sub.subscribed {
		return
	}
	sub.stopCh <- true
	if sub.subscription != nil {
		sub.subscription.Unsubscribe()
		sub.subscription = nil
	}
	sub.subscribed = false
	sub.stopCh = nil
}
