package rabbitmq

import (
	"github.com/streadway/amqp"
)

type EventHandler interface {
	Handle(msg *amqp.Delivery)
}
