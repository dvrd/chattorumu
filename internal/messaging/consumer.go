package messaging

import (
	"context"
	"encoding/json"
	"log"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/service"
	"jobsity-chat/internal/websocket"
)

// ResponseConsumer consumes stock responses and broadcasts them to WebSocket clients
type ResponseConsumer struct {
	rmq         *RabbitMQ
	hub         *websocket.Hub
	chatService *service.ChatService
	botUserID   string
}

// NewResponseConsumer creates a new response consumer
func NewResponseConsumer(rmq *RabbitMQ, hub *websocket.Hub, chatService *service.ChatService, botUserID string) *ResponseConsumer {
	return &ResponseConsumer{
		rmq:         rmq,
		hub:         hub,
		chatService: chatService,
		botUserID:   botUserID,
	}
}

// Start starts consuming stock responses
func (c *ResponseConsumer) Start(ctx context.Context) error {
	// For simplicity, we'll consume from a general responses queue
	// In production, you might want to consume per-chatroom or use a different approach

	// Declare a queue for this consumer
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

	// Bind to responses exchange
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

	log.Println("Started consuming stock responses")

	// Process messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Stopping response consumer")
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Println("Response consumer channel closed")
					return
				}

				var response StockResponse
				if err := json.Unmarshal(msg.Body, &response); err != nil {
					log.Printf("Error unmarshaling response: %v", err)
					continue
				}

				c.processResponse(ctx, &response)
			}
		}
	}()

	return nil
}

func (c *ResponseConsumer) processResponse(ctx context.Context, response *StockResponse) {
	// Determine content based on error
	content := response.FormattedMessage
	if response.Error != "" {
		content = "Error: " + response.Error
	}

	// Save bot message to database
	message := &domain.Message{
		ChatroomID: response.ChatroomID,
		UserID:     c.botUserID,
		Username:   "StockBot",
		Content:    content,
		IsBot:      true,
	}

	if err := c.chatService.SendMessage(ctx, message); err != nil {
		log.Printf("Error saving bot message: %v", err)
		return
	}

	// Broadcast to WebSocket clients
	serverMsg := websocket.ServerMessage{
		Type:      "chat_message",
		ID:        message.ID,
		UserID:    message.UserID,
		Username:  message.Username,
		Content:   message.Content,
		IsBot:     message.IsBot,
		CreatedAt: message.CreatedAt,
	}

	if data, err := json.Marshal(serverMsg); err == nil {
		c.hub.Broadcast(response.ChatroomID, data)
	}
}
