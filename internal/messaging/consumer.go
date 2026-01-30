package messaging

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"jobsity-chat/internal/service"
	"jobsity-chat/internal/websocket"
)

type ResponseConsumer struct {
	rmq         *RabbitMQ
	hub         *websocket.Hub
	chatService *service.ChatService
	botUserID   string
}

func NewResponseConsumer(rmq *RabbitMQ, hub *websocket.Hub, chatService *service.ChatService, botUserID string) *ResponseConsumer {
	return &ResponseConsumer{
		rmq:         rmq,
		hub:         hub,
		chatService: chatService,
		botUserID:   botUserID,
	}
}

func (c *ResponseConsumer) Start(ctx context.Context) error {
	queue, err := c.rmq.channel.QueueDeclare(
		"",    // auto-generated name
		false, // durable
		true,  // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	if err := c.rmq.channel.QueueBind(
		queue.Name,       // queue name
		"",               // routing key
		"chat.responses", // exchange
		false,
		nil,
	); err != nil {
		return err
	}

	msgs, err := c.rmq.channel.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return err
	}

	slog.Info("started consuming stock responses",
		slog.String("queue", queue.Name),
		slog.String("exchange", "chat.responses"))

	go func() {
		for {
			select {
			case <-ctx.Done():
				slog.Info("stopping response consumer")
				return
			case msg, ok := <-msgs:
				if !ok {
					slog.Warn("response consumer channel closed")
					return
				}

				slog.Info("received stock response from queue",
					slog.Int("body_size", len(msg.Body)))

				var response StockResponse
				if err := json.Unmarshal(msg.Body, &response); err != nil {
					slog.Error("error unmarshaling response",
						slog.String("error", err.Error()),
						slog.String("body", string(msg.Body)))
					continue
				}

				slog.Info("processing stock response",
					slog.String("chatroom_id", response.ChatroomID),
					slog.String("symbol", response.Symbol))

				c.processResponse(ctx, &response)
			}
		}
	}()

	return nil
}

func (c *ResponseConsumer) processResponse(_ context.Context, response *StockResponse) {
	content := response.FormattedMessage
	if response.Error != "" {
		content = response.Error
	}

	// Bot messages are NOT saved to database - only broadcast via WebSocket
	slog.Info("processing bot message (not saving to database)",
		slog.String("chatroom_id", response.ChatroomID),
		slog.String("symbol", response.Symbol))

	now := time.Now()
	serverMsg := websocket.ServerMessage{
		Type:      "chat_message",
		ID:        "bot-" + response.ChatroomID + "-" + response.Symbol,
		UserID:    c.botUserID,
		Username:  "StockBot",
		Content:   content,
		IsBot:     true,
		IsError:   response.Error != "",
		CreatedAt: &now,
	}

	if data, err := json.Marshal(serverMsg); err == nil {
		c.hub.Broadcast(response.ChatroomID, data)
		slog.Info("broadcast bot message to websocket",
			slog.String("chatroom_id", response.ChatroomID),
			slog.String("content", content))
	} else {
		slog.Error("error marshaling server message",
			slog.String("error", err.Error()))
	}
}
