package lib

import (
	"sync"
	"time"

	rate "github.com/beefsack/go-rate"
	"github.com/mudkipme/timburr/lib/task"
	"github.com/mudkipme/timburr/utils"
	log "github.com/sirupsen/logrus"
	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

// BasicSubscription can subscribe to kafka topics and handle task to task runner
type BasicSubscription struct {
	config     *SubscriptionConfig
	rule       utils.RuleConfig
	mutex      sync.Mutex
	consumer   *kafka.Consumer
	subscribed bool
	stopChan   chan bool
	limiter    *rate.RateLimiter
}

func (sub *BasicSubscription) topics() []string {
	return ruleTopics(sub.rule)
}

// Subscribe creates a new kafka consumer and start to poll messages
func (sub *BasicSubscription) Subscribe() error {
	sub.mutex.Lock()
	defer sub.mutex.Unlock()
	if sub.subscribed {
		return nil
	}

	var err error
	sub.consumer, err = kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": sub.config.BrokerList,
		"group.id":          sub.config.GroupIDPrefix + sub.rule.Name,
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		return err
	}
	topics := sub.topics()
	err = sub.consumer.SubscribeTopics(topics, nil)
	if err != nil {
		return err
	}
	sub.stopChan = make(chan bool, 1)
	sub.subscribed = true
	if sub.rule.RateLimit > 0 {
		if sub.rule.RateInterval == 0 {
			sub.rule.RateInterval = 1000
		}
		sub.limiter = rate.New(sub.rule.RateLimit, time.Duration(sub.rule.RateInterval)*time.Millisecond)
	}
	go sub.consume()
	log.Infof("subscribed to %v, rule: %v", topics, sub.rule.Name)
	return nil
}

func (sub *BasicSubscription) consume() {
	for sub.subscribed {
		select {
		case <-sub.stopChan:
			sub.mutex.Lock()
			sub.subscribed = false
			sub.mutex.Unlock()
			break
		default:
			ev := sub.consumer.Poll(100)
			if ev == nil {
				continue
			}
			switch e := ev.(type) {
			case *kafka.Message:
				if sub.limiter != nil {
					sub.limiter.Wait()
				}
				sub.mutex.Lock()
				sub.handleMessage(e)
				sub.mutex.Unlock()
			case *kafka.Error:
				if e.Code() == kafka.ErrAllBrokersDown {
					log.WithError(e).Fatal("kafka all broker down")
					sub.mutex.Lock()
					sub.subscribed = false
					sub.mutex.Unlock()
				} else {
					log.WithError(e).Warn("consume message error")
				}
			}
		}
	}
	sub.mutex.Lock()
	err := sub.consumer.Close()
	if err != nil {
		log.WithError(err).Warn("close consumer failed")
	}
	sub.stopChan = nil
	sub.mutex.Unlock()
}

func (sub *BasicSubscription) handleMessage(km *kafka.Message) error {
	executor := task.TypeFromString(sub.rule.TaskType).GetExecutor()
	err := executor.Execute(km.Value)
	if err != nil {
		log.WithError(err).Warn("execute message error")
	}
	return err
}

// Unsubscribe stops polling messages
func (sub *BasicSubscription) Unsubscribe() {
	sub.mutex.Lock()
	defer sub.mutex.Unlock()
	if !sub.subscribed {
		return
	}
	sub.stopChan <- true
}
