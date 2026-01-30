package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

type BotCommand struct {
	Type        string `json:"type"`         // "stock" or "hello"
	ChatroomID  string `json:"chatroom_id"`
	StockCode   string `json:"stock_code,omitempty"`
	RequestedBy string `json:"requested_by"`
	Timestamp   int64  `json:"timestamp"`
}

type StockCommand struct {
	ChatroomID  string `json:"chatroom_id"`
	StockCode   string `json:"stock_code"`
	RequestedBy string `json:"requested_by"`
	Timestamp   int64  `json:"timestamp"`
}

type StockResponse struct {
	ChatroomID       string  `json:"chatroom_id"`
	Symbol           string  `json:"symbol"`
	Price            float64 `json:"price"`
	FormattedMessage string  `json:"formatted_message"`
	Error            string  `json:"error,omitempty"`
	Timestamp        int64   `json:"timestamp"`
}

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

	if err := rmq.Setup(); err != nil {
		rmq.Close()
		return nil, err
	}

	return rmq, nil
}

func (r *RabbitMQ) Setup() error {
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

	if err := r.channel.QueueBind(
		"stock.commands", // queue name
		"stock.request",  // routing key
		"chat.commands",  // exchange
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind stock.commands queue: %w", err)
	}

	slog.Info("rabbitmq setup completed successfully")
	return nil
}

func (r *RabbitMQ) PublishCommand(ctx context.Context, cmd *BotCommand) error {
	body, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		"chat.commands",
		"stock.request",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish command: %w", err)
	}

	slog.Info("published bot command",
		slog.String("type", cmd.Type),
		slog.String("chatroom_id", cmd.ChatroomID))
	return nil
}

func (r *RabbitMQ) PublishStockCommand(ctx context.Context, chatroomID, stockCode, requestedBy string) error {
	cmd := &BotCommand{
		Type:        "stock",
		ChatroomID:  chatroomID,
		StockCode:   stockCode,
		RequestedBy: requestedBy,
		Timestamp:   time.Now().Unix(),
	}
	return r.PublishCommand(ctx, cmd)
}

func (r *RabbitMQ) PublishHelloCommand(ctx context.Context, chatroomID, requestedBy string) error {
	cmd := &BotCommand{
		Type:        "hello",
		ChatroomID:  chatroomID,
		RequestedBy: requestedBy,
		Timestamp:   time.Now().Unix(),
	}
	return r.PublishCommand(ctx, cmd)
}

func (r *RabbitMQ) PublishStockResponse(ctx context.Context, response *StockResponse) error {
	body, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		"chat.responses",
		"",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish response: %w", err)
	}

	slog.Info("published stock response",
		slog.String("symbol", response.Symbol),
		slog.Float64("price", response.Price))
	return nil
}

func (r *RabbitMQ) ConsumeStockCommands() (<-chan amqp.Delivery, error) {
	msgs, err := r.channel.Consume(
		"stock.commands",
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to register consumer: %w", err)
	}

	slog.Info("started consuming stock commands",
		slog.String("queue", "stock.commands"))
	return msgs, nil
}

func (r *RabbitMQ) IsClosed() bool {
	return r.conn == nil || r.conn.IsClosed()
}

func (r *RabbitMQ) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
