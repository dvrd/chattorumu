package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jobsity-chat/internal/config"
	"jobsity-chat/internal/messaging"
	"jobsity-chat/internal/stock"
)

func main() {
	log.Println("Starting Stock Bot...")

	// Load configuration
	cfg := config.Load()

	// Connect to RabbitMQ
	rmq, err := messaging.NewRabbitMQ(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rmq.Close()

	log.Println("Connected to RabbitMQ")

	// Initialize Stooq client
	stooqClient := stock.NewStooqClient(cfg.StooqAPIURL)

	// Start consuming messages
	msgs, err := rmq.ConsumeStockCommands()
	if err != nil {
		log.Fatalf("Failed to start consuming: %v", err)
	}

	log.Println("Stock Bot is ready to process commands")

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Process messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Stopping message consumer")
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Println("Message channel closed")
					return
				}
				// Use context with timeout for processing
				msgCtx, msgCancel := context.WithTimeout(ctx, 30*time.Second)
				if err := processCommand(msgCtx, msg.Body, stooqClient, rmq); err != nil {
					log.Printf("Error processing command: %v", err)
				}
				msgCancel()
				msg.Ack(false)
			}
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down Stock Bot...")
	cancel()
	time.Sleep(1 * time.Second)
	log.Println("Stock Bot stopped")
}

func processCommand(ctx context.Context, body []byte, stooqClient *stock.StooqClient, rmq *messaging.RabbitMQ) error {
	// Parse command
	var cmd messaging.StockCommand
	if err := json.Unmarshal(body, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	log.Printf("Processing stock command: %s (chatroom: %s, requested by: %s)",
		cmd.StockCode, cmd.ChatroomID, cmd.RequestedBy)

	// Fetch quote from Stooq
	quote, err := stooqClient.GetQuote(ctx, cmd.StockCode)

	// Prepare response
	response := &messaging.StockResponse{
		ChatroomID: cmd.ChatroomID,
		Timestamp:  time.Now().Unix(),
	}

	if err != nil {
		// Handle error
		log.Printf("Error fetching quote for %s: %v", cmd.StockCode, err)
		response.Error = fmt.Sprintf("Failed to fetch quote for %s", cmd.StockCode)

		if err == stock.ErrStockNotFound {
			response.Error = fmt.Sprintf("Stock %s not found", cmd.StockCode)
		}
	} else {
		// Success
		response.Symbol = quote.Symbol
		response.Price = quote.Price
		response.FormattedMessage = fmt.Sprintf("%s quote is $%.2f per share", quote.Symbol, quote.Price)
		log.Printf("Successfully fetched quote: %s - $%.2f", quote.Symbol, quote.Price)
	}

	// Publish response
	if err := rmq.PublishStockResponse(ctx, response); err != nil {
		return fmt.Errorf("failed to publish response: %w", err)
	}

	return nil
}
