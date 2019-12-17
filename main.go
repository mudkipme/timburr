package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	logrustash "github.com/bshuster-repo/logrus-logstash-hook"
	log "github.com/sirupsen/logrus"

	"github.com/mudkipme/timburr/lib"
	"github.com/mudkipme/timburr/utils"
)

func main() {
	if err := utils.InitConfig(); err != nil {
		log.WithError(err).Panic("config init failed")
	}

	if utils.Config.Options.Logstash != "" {
		conn, err := net.Dial("tcp", utils.Config.Options.Logstash)
		if err != nil {
			log.Fatal(err)
		}
		hook, err := logrustash.NewHookWithConn(conn, "timburr")
		if err != nil {
			log.Fatal(err)
		}
		log.AddHook(hook)
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
