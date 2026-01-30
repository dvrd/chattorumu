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

var zenPhrases = []string{
	"The obstacle is the path.",
	"Let go or be dragged.",
	"The quieter you become, the more you can hear.",
	"Nature does not hurry, yet everything is accomplished.",
	"When you realize nothing is lacking, the whole world belongs to you.",
	"The journey of a thousand miles begins with a single step.",
	"Be like water, flowing around obstacles.",
	"In the midst of chaos, there is also opportunity.",
	"The wise adapt themselves to circumstances, as water molds itself to the pitcher.",
	"Tension is who you think you should be. Relaxation is who you are.",
	"Empty your mind, be formless, shapeless — like water.",
	"The seed of suffering in you may be strong, but don't wait until you have no more suffering before allowing yourself to be happy.",
	"Walk as if you are kissing the Earth with your feet.",
	"Breathing in, I calm body and mind. Breathing out, I smile.",
	"The present moment is filled with joy and happiness. If you are attentive, you will see it.",
	"Wherever you are, be there totally.",
	"Realize deeply that the present moment is all you have.",
	"Accept — then act. Whatever the present moment contains, accept it as if you had chosen it.",
	"The primary cause of unhappiness is never the situation but your thoughts about it.",
	"Life is a balance of holding on and letting go.",
	"Sometimes you need to step outside, get some air, and remind yourself of who you are and where you want to be.",
	"The only Zen you find on tops of mountains is the Zen you bring there.",
	"Before enlightenment: chop wood, carry water. After enlightenment: chop wood, carry water.",
	"Let things flow naturally forward in whatever way they like.",
	"Do not seek the truth, only cease to cherish your opinions.",
	"When the student is ready, the teacher appears.",
	"The cave you fear to enter holds the treasure you seek.",
	"Silence is the language of the wise.",
	"The mind is everything. What you think you become.",
	"Peace comes from within. Do not seek it without.",
	"No snowflake ever falls in the wrong place.",
	"Knowledge is learning something new every day. Wisdom is letting go of something every day.",
	"In the beginner's mind there are many possibilities, but in the expert's there are few.",
	"If you understand, things are just as they are. If you do not understand, things are just as they are.",
	"The moon does not fight. It attacks no one. It does not worry. It does not try to crush others.",
	"Sitting quietly, doing nothing, spring comes, and the grass grows by itself.",
	"The snow falls, each flake in its appropriate place.",
	"To a mind that is still, the whole universe surrenders.",
	"Muddy water is best cleared by leaving it alone.",
	"The best time to plant a tree was 20 years ago. The second best time is now.",
	"A single arrow is easily broken, but not ten in a bundle.",
	"The bamboo that bends is stronger than the oak that resists.",
	"Where there is no desire, there is stillness.",
	"The flame that burns twice as bright burns half as long.",
	"Be master of mind rather than mastered by mind.",
	"Flow with whatever may happen and let your mind be free.",
	"The wise man knows he doesn't know. The fool thinks he knows all.",
	"Inner peace begins the moment you choose not to allow another person or event to control your emotions.",
	"Patience is not about waiting, but the ability to keep a good attitude while working hard.",
	"The root of suffering is attachment.",
}

func processCommand(ctx context.Context, body []byte, stooqClient *stock.StooqClient, rmq *messaging.RabbitMQ) error {
	// Try parsing as BotCommand first
	var cmd messaging.BotCommand
	if err := json.Unmarshal(body, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	slog.Info("processing bot command",
		slog.String("type", cmd.Type),
		slog.String("chatroom_id", cmd.ChatroomID),
		slog.String("requested_by", cmd.RequestedBy))

	// Prepare response
	response := &messaging.StockResponse{
		ChatroomID: cmd.ChatroomID,
		Timestamp:  time.Now().Unix(),
	}

	switch cmd.Type {
	case "stock":
		// Fetch quote from Stooq
		quote, err := stooqClient.GetQuote(ctx, cmd.StockCode)

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

	case "hello":
		// Select random zen phrase
		phrase := zenPhrases[time.Now().UnixNano()%int64(len(zenPhrases))]
		response.FormattedMessage = phrase
		response.Symbol = "zen"
		slog.Info("sending zen phrase",
			slog.String("phrase", phrase))

	default:
		response.Error = fmt.Sprintf("Unknown command type: %s", cmd.Type)
		slog.Warn("unknown command type", slog.String("type", cmd.Type))
	}

	// Publish response
	if err := rmq.PublishStockResponse(ctx, response); err != nil {
		return fmt.Errorf("failed to publish response: %w", err)
	}

	return nil
}
