package service

import (
	"testing"
)

func TestParseCommand_ValidStockCommand(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedType  string
		expectedStock string
	}{
		{
			name:          "simple stock code",
			input:         "/stock=AAPL.US",
			expectedType:  "stock",
			expectedStock: "AAPL.US",
		},
		{
			name:          "lowercase converted to uppercase",
			input:         "/stock=aapl.us",
			expectedType:  "stock",
			expectedStock: "AAPL.US",
		},
		{
			name:          "mixed case",
			input:         "/stock=AaPl.Us",
			expectedType:  "stock",
			expectedStock: "AAPL.US",
		},
		{
			name:          "stock with numbers",
			input:         "/stock=BRK.B",
			expectedType:  "stock",
			expectedStock: "BRK.B",
		},
		{
			name:          "single letter stock",
			input:         "/stock=X",
			expectedType:  "stock",
			expectedStock: "X",
		},
		{
			name:          "long stock code",
			input:         "/stock=ABCDEFGHIJ.12345678",
			expectedType:  "stock",
			expectedStock: "ABCDEFGHIJ.12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, isCommand := ParseCommand(tt.input)

			if !isCommand {
				t.Errorf("Expected to be recognized as command")
			}

			if cmd == nil {
				t.Fatal("Expected non-nil command")
			}

			if cmd.Type != tt.expectedType {
				t.Errorf("Expected type %q, got %q", tt.expectedType, cmd.Type)
			}

			if cmd.StockCode != tt.expectedStock {
				t.Errorf("Expected stock code %q, got %q", tt.expectedStock, cmd.StockCode)
			}
		})
	}
}

func TestParseCommand_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing stock code",
			input: "/stock=",
		},
		{
			name:  "no equals sign",
			input: "/stock",
		},
		{
			name:  "regular message",
			input: "Hello, world!",
		},
		{
			name:  "stock without slash",
			input: "stock=AAPL.US",
		},
		{
			name:  "stock with space",
			input: "/stock=AAPL US",
		},
		{
			name:  "stock too long (over 20 chars)",
			input: "/stock=ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		},
		{
			name:  "stock with special characters",
			input: "/stock=AAPL@US",
		},
		{
			name:  "stock with underscore",
			input: "/stock=AAPL_US",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "just slash",
			input: "/",
		},
		{
			name:  "multiple equals",
			input: "/stock=AAPL=US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, isCommand := ParseCommand(tt.input)

			if isCommand {
				t.Errorf("Expected NOT to be recognized as command, got: %+v", cmd)
			}

			if cmd != nil {
				t.Errorf("Expected nil command, got: %+v", cmd)
			}
		})
	}
}

func TestParseCommand_WithWhitespace(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		shouldParse   bool
		expectedStock string
	}{
		{
			name:          "leading whitespace",
			input:         "  /stock=AAPL.US",
			shouldParse:   true,
			expectedStock: "AAPL.US",
		},
		{
			name:          "trailing whitespace",
			input:         "/stock=AAPL.US  ",
			shouldParse:   true,
			expectedStock: "AAPL.US",
		},
		{
			name:          "both leading and trailing",
			input:         "  /stock=AAPL.US  ",
			shouldParse:   true,
			expectedStock: "AAPL.US",
		},
		{
			name:        "whitespace in the middle",
			input:       "/stock= AAPL.US",
			shouldParse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, isCommand := ParseCommand(tt.input)

			if isCommand != tt.shouldParse {
				t.Errorf("Expected shouldParse=%v, got isCommand=%v", tt.shouldParse, isCommand)
			}

			if tt.shouldParse && cmd != nil {
				if cmd.StockCode != tt.expectedStock {
					t.Errorf("Expected stock code %q, got %q", tt.expectedStock, cmd.StockCode)
				}
			}
		})
	}
}

func TestParseCommand_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldParse bool
	}{
		{
			name:        "tab character",
			input:       "\t/stock=AAPL.US",
			shouldParse: true,
		},
		{
			name:        "newline",
			input:       "/stock=AAPL.US\n",
			shouldParse: true,
		},
		{
			name:        "multiple newlines",
			input:       "\n\n/stock=AAPL.US\n\n",
			shouldParse: true,
		},
		{
			name:        "carriage return",
			input:       "/stock=AAPL.US\r\n",
			shouldParse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, isCommand := ParseCommand(tt.input)

			if isCommand != tt.shouldParse {
				t.Errorf("Expected shouldParse=%v, got isCommand=%v", tt.shouldParse, isCommand)
			}

			if tt.shouldParse && cmd == nil {
				t.Error("Expected non-nil command for valid input")
			}
		})
	}
}

func TestParseCommand_ReturnValues(t *testing.T) {
	t.Run("valid command returns non-nil command and true", func(t *testing.T) {
		cmd, isCommand := ParseCommand("/stock=AAPL.US")

		if !isCommand {
			t.Error("Expected isCommand to be true")
		}

		if cmd == nil {
			t.Error("Expected non-nil command")
		}

		if cmd != nil {
			if cmd.Type == "" {
				t.Error("Expected Type to be set")
			}
			if cmd.StockCode == "" {
				t.Error("Expected StockCode to be set")
			}
		}
	})

	t.Run("invalid command returns nil and false", func(t *testing.T) {
		cmd, isCommand := ParseCommand("Hello, world!")

		if isCommand {
			t.Error("Expected isCommand to be false")
		}

		if cmd != nil {
			t.Errorf("Expected nil command, got: %+v", cmd)
		}
	})
}

// Benchmark tests
func BenchmarkParseCommand_Valid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseCommand("/stock=AAPL.US")
	}
}

func BenchmarkParseCommand_Invalid(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseCommand("Hello, world!")
	}
}

func BenchmarkParseCommand_WithWhitespace(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseCommand("  /stock=AAPL.US  ")
	}
}
