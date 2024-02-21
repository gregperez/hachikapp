package subscription

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"jwt/base/internal/messaging/rabbitmq"
	rand2 "math/rand"
	"sync"
	"time"
)

var (
	testWrapper    *TestSubscription
	testWrapperOne sync.Once
)

type TestSubscription struct {
}

func NewTestSubscription() *TestSubscription {
	testWrapperOne.Do(func() {
		testWrapper = &TestSubscription{}
	})
	return testWrapper
}

func (t *TestSubscription) Start() {
	rabbitmq.InitSubscriber("hola", "amqp", "admin", "admin", "localhost", "5672", 10, t)
}

func (t *TestSubscription) Handle(msg *amqp.Delivery) {
	fmt.Printf("Received message: %s", msg.Body)
	// Simular algún tipo de procesamiento
	time.Sleep(2 * time.Second)
	// randon
	rand := rand2.Int()
	// Si ocurre un error, devolver el mensaje a la cola
	if rand%2 == 0 { // Coloca aquí tu lógica de manejo de errores
		log.Printf("Ocurrió un error. Devolviendo el mensaje a la cola.")
		if err := msg.Nack(false, true); err != nil {
			log.Printf("Error al devolver el mensaje a la cola: %s", err)
		}
	} else {
		// Acknowledge manual del mensaje si el procesamiento fue exitoso
		if err := msg.Ack(false); err != nil {
			log.Printf("Error al enviar Acknowledgement: %s", err)
		}
	}
}
