package main

import (
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitPublisher holds 1 connection, 1 channel (as recommended by RabbitMQ docs)
type RabbitPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex // serialize publishes → channels are not thread-safe
}

// NewRabbitPublisher creates and returns a connected publisher.
func NewRabbitPublisher(amqpURL string) (*RabbitPublisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Let every queue be declared "durable" by default.
	return &RabbitPublisher{
		conn:    conn,
		channel: ch,
	}, nil
}

// Publish implements the PublishFunc signature
func (p *RabbitPublisher) Publish(destination string, body []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Declare queue idempotently (safe even if already declared)
	_, err := p.channel.QueueDeclare(
		destination,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
	if err != nil {
		return err
	}

	// Send message
	err = p.channel.Publish(
		"",          // direct queue publish (no exchange)
		destination, // queue name
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent, // IMPORTANT: persist message
			ContentType:  "application/json",
			Body:         body,
		},
	)
	if err != nil {
		return err
	}

	log.Printf("[rabbitmq] published to %s (%d bytes)", destination, len(body))
	return nil
}

func (p *RabbitPublisher) Close() {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
}
