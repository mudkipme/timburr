package server

import (
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

type ServerConfig struct {
	BrokerList   string
	Listen       string
	TopicKey     string
	DefaultTopic string
}

type TimburrServer struct {
	producer *kafka.Producer
	server   *http.Server
	config   *ServerConfig
}

func NewTimburrServer(config *ServerConfig) (*TimburrServer, error) {
	s := &TimburrServer{
		config: config,
	}
	if err := s.setup(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *TimburrServer) setup() error {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": s.config.BrokerList,
	})
	if err != nil {
		return err
	}
	s.producer = p

	s.server = &http.Server{Addr: s.config.Listen, Handler: s}
	return nil
}

func (s *TimburrServer) sendMessage(data gjson.Result) error {
	topic := data.Get(s.config.TopicKey).String()
	if topic == "" {
		topic = s.config.DefaultTopic
	}

	deliveryChan := make(chan kafka.Event, 1)
	err := s.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          []byte(data.Raw),
	}, deliveryChan)
	if err != nil {
		log.WithError(err).Warn("http produce error")
		return err
	}

	e := <-deliveryChan
	m := e.(*kafka.Message)
	if m.TopicPartition.Error != nil {
		log.WithError(err).Warn("http produce error")
		close(deliveryChan)
		return err
	}
	close(deliveryChan)
	return nil
}

func (s *TimburrServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		return
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("read request failed"))
		return
	}

	json := gjson.Parse(string(body))
	if json.IsArray() {
		json.ForEach(func(key, value gjson.Result) bool {
			if err = s.sendMessage(value); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("send to kafka error: " + err.Error()))
				return false
			}
			return true
		})
	} else {
		if err = s.sendMessage(json); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("send to kafka error: " + err.Error()))
			return
		}
	}

	if err == nil {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *TimburrServer) Start() error {
	log.Infof("server start at %s", s.config.Listen)
	return s.server.ListenAndServe()
}

func (s *TimburrServer) Close() error {
	s.producer.Flush(10000)
	s.producer.Close()
	return s.server.Close()
}
