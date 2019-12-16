package main

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/mudkipme/timburr/lib"
	"github.com/mudkipme/timburr/utils"
)

func main() {
	if err := utils.InitConfig(); err != nil {
		log.WithError(err).Panic("config init failed")
	}

	sub := lib.DefaultSubscriber()
	for _, rule := range utils.Config.Rules {
		if err := sub.Subscribe(rule); err != nil {
			log.WithError(err).Panicf("subscribe rule %v failed", rule.Name)
		}
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	run := true
	for run {
		select {
		case <-sigchan:
			sub.Unsubscribe()
			run = false
		}
	}
}
