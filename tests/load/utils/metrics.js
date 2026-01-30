import { Counter, Trend, Rate } from 'k6/metrics';

// Custom metrics for WebSocket chat
export const messagesReceived = new Counter('chat_messages_received');
export const messagesSent = new Counter('chat_messages_sent');
export const stockCommandsSent = new Counter('stock_commands_sent');
export const stockResponsesReceived = new Counter('stock_responses_received');
export const messageLatency = new Trend('chat_message_latency_ms');
export const stockCommandLatency = new Trend('stock_command_latency_ms');
export const websocketErrors = new Counter('websocket_errors');
export const connectionSuccess = new Rate('websocket_connection_success');
export const messageDeliveryRate = new Rate('message_delivery_success');
