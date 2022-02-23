package main

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/mudkipme/timburr/lib"
	"github.com/mudkipme/timburr/server"
	"github.com/mudkipme/timburr/utils"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	if err := utils.InitConfig(); err != nil {
		log.WithError(err).Panic("config init failed")
	}

	server, err := server.NewTimburrServer(&server.ServerConfig{
		BrokerList:   utils.Config.Kafka.BrokerList,
		Listen:       utils.Config.Options.Listen,
		TopicKey:     utils.Config.Options.TopicKey,
		DefaultTopic: utils.Config.Options.DefaultTopic,
	})
	if err != nil {
		log.WithError(err).Panic("create server failed")
	}

	go server.Start()

	sub := lib.DefaultSubscriber()
	for _, rule := range utils.Config.Rules {
		if err := sub.Subscribe(rule); err != nil {
			log.WithError(err).Panicf("subscribe rule %v failed", rule.Name)
		}
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	<-sigchan
	sub.Unsubscribe()
	server.Close()
}
