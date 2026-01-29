package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQ manages RabbitMQ connection and operations
type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// StockCommand represents a stock quote request
type StockCommand struct {
	ChatroomID  string `json:"chatroom_id"`
	StockCode   string `json:"stock_code"`
	RequestedBy string `json:"requested_by"`
	Timestamp   int64  `json:"timestamp"`
}

// StockResponse represents a stock quote response
type StockResponse struct {
	ChatroomID       string  `json:"chatroom_id"`
	Symbol           string  `json:"symbol"`
	Price            float64 `json:"price"`
	FormattedMessage string  `json:"formatted_message"`
	Error            string  `json:"error,omitempty"`
	Timestamp        int64   `json:"timestamp"`
}

// NewRabbitMQ creates a new RabbitMQ connection
func NewRabbitMQ(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	rmq := &RabbitMQ{
		conn:    conn,
		channel: ch,
	}

	// Declare exchanges and queues
	if err := rmq.Setup(); err != nil {
		rmq.Close()
		return nil, err
	}

	return rmq, nil
}

// Setup declares exchanges and queues
func (r *RabbitMQ) Setup() error {
	// Declare commands exchange (topic)
	if err := r.channel.ExchangeDeclare(
		"chat.commands", // name
		"topic",         // type
		true,            // durable
		false,           // auto-deleted
		false,           // internal
		false,           // no-wait
		nil,             // arguments
	); err != nil {
		return fmt.Errorf("failed to declare commands exchange: %w", err)
	}

	// Declare responses exchange (fanout)
	if err := r.channel.ExchangeDeclare(
		"chat.responses", // name
		"fanout",         // type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	); err != nil {
		return fmt.Errorf("failed to declare responses exchange: %w", err)
	}

	// Declare stock commands queue
	if _, err := r.channel.QueueDeclare(
		"stock.commands", // name
		true,             // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	); err != nil {
		return fmt.Errorf("failed to declare stock.commands queue: %w", err)
	}

	// Bind stock commands queue to exchange
	if err := r.channel.QueueBind(
		"stock.commands", // queue name
		"stock.request",  // routing key
		"chat.commands",  // exchange
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind stock.commands queue: %w", err)
	}

	log.Println("RabbitMQ setup completed successfully")
	return nil
}

// PublishStockCommand publishes a stock command request
func (r *RabbitMQ) PublishStockCommand(ctx context.Context, chatroomID, stockCode, requestedBy string) error {
	cmd := StockCommand{
		ChatroomID:  chatroomID,
		StockCode:   stockCode,
		RequestedBy: requestedBy,
		Timestamp:   time.Now().Unix(),
	}

	body, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		"chat.commands", // exchange
		"stock.request", // routing key
		false,           // mandatory
		false,           // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish command: %w", err)
	}

	log.Printf("Published stock command: %s for chatroom %s", stockCode, chatroomID)
	return nil
}

// PublishStockResponse publishes a stock quote response
func (r *RabbitMQ) PublishStockResponse(ctx context.Context, response *StockResponse) error {
	body, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		"chat.responses", // exchange
		"",               // routing key (ignored for fanout)
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish response: %w", err)
	}

	log.Printf("Published stock response: %s - $%.2f", response.Symbol, response.Price)
	return nil
}

// ConsumeStockCommands sets up a consumer for stock commands
func (r *RabbitMQ) ConsumeStockCommands() (<-chan amqp.Delivery, error) {
	msgs, err := r.channel.Consume(
		"stock.commands", // queue
		"",               // consumer
		false,            // auto-ack (false for manual ack)
		false,            // exclusive
		false,            // no-local
		false,            // no-wait
		nil,              // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register consumer: %w", err)
	}

	log.Println("Started consuming stock commands")
	return msgs, nil
}

// ConsumeStockResponses sets up a consumer for stock responses
func (r *RabbitMQ) ConsumeStockResponses(chatroomID string) (<-chan amqp.Delivery, error) {
	// Declare a unique queue for this chatroom
	queueName := fmt.Sprintf("stock.responses.%s", chatroomID)
	queue, err := r.channel.QueueDeclare(
		queueName, // name
		false,     // durable
		true,      // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare response queue: %w", err)
	}

	// Bind to responses exchange
	if err := r.channel.QueueBind(
		queue.Name,       // queue name
		"",               // routing key
		"chat.responses", // exchange
		false,
		nil,
	); err != nil {
		return nil, fmt.Errorf("failed to bind response queue: %w", err)
	}

	msgs, err := r.channel.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register consumer: %w", err)
	}

	log.Printf("Started consuming stock responses for chatroom %s", chatroomID)
	return msgs, nil
}

// IsClosed returns true if the connection is closed
func (r *RabbitMQ) IsClosed() bool {
	return r.conn == nil || r.conn.IsClosed()
}

// Close closes the RabbitMQ connection
func (r *RabbitMQ) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
