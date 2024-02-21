package rabbitmq

import (
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type RabbitMQWrapper struct {
	channel *amqp.Channel
	queue   *amqp.Queue
	consume <-chan amqp.Delivery
}

func InitSubscriber(queue string, protocol string, user string, pass string, host string, port string, prefetchCount int, event EventHandler) {
	client, err := NewChannel(protocol, user, pass, host, port)

	if err != nil {
		log.Fatalf("Failed to create client: %v\n", err)
	}

	client.Consume(queue, prefetchCount)

	log.Printf("Start receiving messages, queue=%s MaxMessages=%v", queue, prefetchCount)

	client.Receive(event)
}

func NewChannel(protocol string, user string, pass string, host string, port string) (*RabbitMQWrapper, error) {
	conn, err := amqp.Dial(fmt.Sprintf("%s://%s:%s@%s:%s?hearbeat=60", protocol, user, pass, host, port))
	if err != nil {
		log.Fatalf("Error connecting to RabbitMQ: %s", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Error opening channel: %s", err)
	}

	return &RabbitMQWrapper{channel: ch}, nil
}

func (rw *RabbitMQWrapper) CloseChannel() error {
	err := rw.channel.Close()
	if err != nil {
		log.Printf("Failed to close client: %v\n", err)
	}
	return err
}

func (rw *RabbitMQWrapper) Consume(queue string, prefetchCount int) {
	// Configurar el prefetch count para este canal.
	err := rw.channel.Qos(
		prefetchCount, // prefetch count
		0,             // prefetch size (0 significa sin límite específico en el tamaño del mensaje)
		false,         // global: true o false (aplica el setting a nivel de canal o consumidor)
	)
	if err != nil {
		log.Fatalf("Failed to set QoS: %s", err)
	}

	consume, err := rw.channel.Consume(
		queue, //q.Name, // Nombre de la cola
		"",    // Identificador del consumidor (vacío para uno generado por RabbitMQ)
		false, // No es auto-acknowledged
		false, // No es exclusivo
		false, // No espera mensajes adicionales
		false, // No se esperan propiedades adicionales
		nil,   // Argumentos adicionales
	)
	if err != nil {
		log.Fatalf("Error registering consumer: %s", err)
	}

	rw.consume = consume
}

func (rw *RabbitMQWrapper) Publish(exchange string, queue string, payload []byte, attr map[string]string) error {
	err := rw.channel.Publish(
		exchange, // Exchange
		queue,    // Queue
		false,    // Mandatory
		false,    // Immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        payload,
		})
	if err != nil {
		log.Printf("Failed to publish a message: %v\n", err)
	}
	return err
}

func (rw *RabbitMQWrapper) Receive(event EventHandler) {
	go func() {
		for d := range rw.consume {
			fmt.Printf("Message received: %s\n", d.Body)
			event.Handle(&d)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}
