package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jobsity-chat/internal/config"
	"jobsity-chat/internal/messaging"
	"jobsity-chat/internal/observability"
	"jobsity-chat/internal/stock"
)

func main() {
	// Load configuration first
	cfg := config.Load()

	// Initialize structured logging
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "json"
	}
	observability.InitLogger(logLevel, logFormat)

	slog.Info("starting stock bot")

	// Connect to RabbitMQ
	rmq, err := messaging.NewRabbitMQ(cfg.RabbitMQURL)
	if err != nil {
		slog.Error("failed to connect to rabbitmq", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer rmq.Close()

	slog.Info("connected to rabbitmq")

	// Initialize Stooq client
	stooqClient := stock.NewStooqClient(cfg.StooqAPIURL)

	// Start consuming messages
	msgs, err := rmq.ConsumeStockCommands()
	if err != nil {
		slog.Error("failed to start consuming", slog.String("error", err.Error()))
		os.Exit(1)
	}

	slog.Info("stock bot is ready to process commands")

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
				slog.Info("stopping message consumer")
				return
			case msg, ok := <-msgs:
				if !ok {
					slog.Info("message channel closed")
					return
				}
				// Use context with timeout for processing
				msgCtx, msgCancel := context.WithTimeout(ctx, 30*time.Second)
				if err := processCommand(msgCtx, msg.Body, stooqClient, rmq); err != nil {
					slog.Error("error processing command", slog.String("error", err.Error()))
				}
				msgCancel()
				msg.Ack(false)
			}
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	slog.Info("shutting down stock bot")
	cancel()
	time.Sleep(1 * time.Second)
	slog.Info("stock bot stopped")
}

func processCommand(ctx context.Context, body []byte, stooqClient *stock.StooqClient, rmq *messaging.RabbitMQ) error {
	// Parse command
	var cmd messaging.StockCommand
	if err := json.Unmarshal(body, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	slog.Info("processing stock command",
		slog.String("stock_code", cmd.StockCode),
		slog.String("chatroom_id", cmd.ChatroomID),
		slog.String("requested_by", cmd.RequestedBy))

	// Fetch quote from Stooq
	quote, err := stooqClient.GetQuote(ctx, cmd.StockCode)

	// Prepare response
	response := &messaging.StockResponse{
		ChatroomID: cmd.ChatroomID,
		Timestamp:  time.Now().Unix(),
	}

	if err != nil {
		// Handle error
		slog.Error("error fetching quote",
			slog.String("stock_code", cmd.StockCode),
			slog.String("error", err.Error()))
		response.Error = fmt.Sprintf("Failed to fetch quote for %s", cmd.StockCode)

		if err == stock.ErrStockNotFound {
			response.Error = fmt.Sprintf("Stock %s not found", cmd.StockCode)
		}
	} else {
		// Success
		response.Symbol = quote.Symbol
		response.Price = quote.Price
		response.FormattedMessage = fmt.Sprintf("%s quote is $%.2f per share", quote.Symbol, quote.Price)
		slog.Info("successfully fetched quote",
			slog.String("symbol", quote.Symbol),
			slog.Float64("price", quote.Price))
	}

	// Publish response
	if err := rmq.PublishStockResponse(ctx, response); err != nil {
		return fmt.Errorf("failed to publish response: %w", err)
	}

	return nil
}
