package prop

import (
	"log"
	"sync"
)

var (
	queueOnce sync.Once
	queueProp *QueueProp
)

type QueueProp struct {
	Queue Queue `yaml:"queues"`
}

type Queue struct {
	RabbitMQ RabbitMQ `yaml:"rabbitmq"`
}

type RabbitMQ struct {
	NameQueue     string `yaml:"name-queue"`
	Protocol      string `yaml:"protocol"`
	User          string `yaml:"user"`
	Pass          string `yaml:"pass"`
	Host          string `yaml:"host"`
	Port          string `yaml:"port"`
	PreFetchCount int    `yaml:"prefetch-count"`
}

func GetQueueProp() *QueueProp {
	queueOnce.Do(func() {
		queueProp = &QueueProp{}
	})
	return queueProp
}

func (d *QueueProp) Validate() {
	if d.Queue.RabbitMQ.NameQueue == "" {
		log.Fatal("Queue.RabbitMQ.NameQueue is empty")
	}
}
