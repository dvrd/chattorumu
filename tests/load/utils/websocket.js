import ws from 'k6/experimental/websockets';
import { check } from 'k6';
import * as metrics from './metrics.js';

const BASE_WS_URL = __ENV.BASE_WS_URL || 'ws://chat-server:8080';

/**
 * Generate random chat messages
 */
const CHAT_MESSAGES = [
  'Hello everyone!',
  'How are you doing?',
  'This is a test message',
  'Great weather today',
  'Anyone here?',
  'Testing the chat system',
  'Load test in progress',
  'Hello from k6!',
  'Message number ',
  'Chat is working well',
];

const STOCK_SYMBOLS = [
  'aapl.us',
  'googl.us',
  'msft.us',
  'amzn.us',
  'tsla.us',
  'fb.us',
  'nflx.us',
];

/**
 * Get random chat message
 * @returns {string} Random message
 */
export function getRandomMessage() {
  const msg = CHAT_MESSAGES[Math.floor(Math.random() * CHAT_MESSAGES.length)];
  return `${msg} ${Date.now()}`;
}

/**
 * Get random stock command
 * @returns {string} Stock command
 */
export function getRandomStockCommand() {
  const symbol = STOCK_SYMBOLS[Math.floor(Math.random() * STOCK_SYMBOLS.length)];
  return `/stock=${symbol}`;
}

/**
 * Connect to WebSocket and handle chat session
 * @param {string} chatroomId - Chatroom ID
 * @param {string} token - Auth token
 * @param {Object} options - Configuration options
 * @param {number} options.duration - Connection duration in seconds
 * @param {number} options.messageInterval - Time between messages in ms
 * @param {number} options.stockCommandInterval - Time between stock commands in ms (0 to disable)
 * @param {function} options.onMessage - Custom message handler
 * @returns {Object} Session statistics
 */
export function chatSession(chatroomId, token, options = {}) {
  const {
    duration = 60,
    messageInterval = 5000,
    stockCommandInterval = 0,
    onMessage = null,
  } = options;

  const url = `${BASE_WS_URL}/ws?chatroom_id=${chatroomId}&token=${token}`;

  const stats = {
    connected: false,
    messagesSent: 0,
    messagesReceived: 0,
    errors: 0,
  };

  const sentMessages = new Map(); // Track sent message timestamps for latency

  const response = ws.connect(url, {}, function(socket) {
    socket.on('open', () => {
      stats.connected = true;
      metrics.connectionSuccess.add(1);

      // Send regular chat messages
      const messageTimer = socket.setInterval(() => {
        const content = getRandomMessage();
        const timestamp = Date.now();
        const msg = {
          type: 'chat_message',
          content: content,
          timestamp: timestamp,
        };

        sentMessages.set(content, timestamp);
        socket.send(JSON.stringify(msg));
        stats.messagesSent++;
        metrics.messagesSent.add(1);
      }, messageInterval);

      // Send stock commands if enabled
      let stockTimer;
      if (stockCommandInterval > 0) {
        stockTimer = socket.setInterval(() => {
          const command = getRandomStockCommand();
          const timestamp = Date.now();
          const msg = {
            type: 'chat_message',
            content: command,
            timestamp: timestamp,
          };

          sentMessages.set(command, timestamp);
          socket.send(JSON.stringify(msg));
          stats.messagesSent++;
          metrics.messagesSent.add(1);
          metrics.stockCommandsSent.add(1);
        }, stockCommandInterval);
      }

      // Close connection after duration
      socket.setTimeout(() => {
        if (messageTimer) socket.clearInterval(messageTimer);
        if (stockTimer) socket.clearInterval(stockTimer);
        socket.close();
      }, duration * 1000);
    });

    socket.on('message', (data) => {
      try {
        const msg = JSON.parse(data);
        stats.messagesReceived++;
        metrics.messagesReceived.add(1);

        // Calculate latency if we sent this message
        if (msg.content && sentMessages.has(msg.content)) {
          const sentTime = sentMessages.get(msg.content);
          const latency = Date.now() - sentTime;

          if (msg.content.startsWith('/stock=')) {
            metrics.stockCommandLatency.add(latency);
            metrics.stockResponsesReceived.add(1);
          } else {
            metrics.messageLatency.add(latency);
          }

          sentMessages.delete(msg.content);
        }

        // Custom message handler
        if (onMessage) {
          onMessage(msg);
        }

        metrics.messageDeliveryRate.add(1);
      } catch (e) {
        console.error(`Failed to parse message: ${e}`);
        metrics.messageDeliveryRate.add(0);
      }
    });

    socket.on('error', (e) => {
      console.error(`WebSocket error: ${e.error()}`);
      stats.errors++;
      metrics.websocketErrors.add(1);
      metrics.connectionSuccess.add(0);
    });

    socket.on('close', () => {
      // Connection closed
    });
  });

  check(response, {
    'websocket connected successfully': (r) => r && r.status === 101,
  });

  return stats;
}

/**
 * Simple connection test - just connect and disconnect
 * @param {string} chatroomId - Chatroom ID
 * @param {string} token - Auth token
 * @param {number} duration - How long to stay connected (seconds)
 * @returns {boolean} Success status
 */
export function connectAndHold(chatroomId, token, duration = 30) {
  const url = `${BASE_WS_URL}/ws?chatroom_id=${chatroomId}&token=${token}`;

  let connected = false;

  const response = ws.connect(url, {}, function(socket) {
    socket.on('open', () => {
      connected = true;
      metrics.connectionSuccess.add(1);

      socket.setTimeout(() => {
        socket.close();
      }, duration * 1000);
    });

    socket.on('error', (e) => {
      console.error(`Connection error: ${e.error()}`);
      metrics.websocketErrors.add(1);
      metrics.connectionSuccess.add(0);
    });
  });

  return check(response, {
    'websocket connected': (r) => r && r.status === 101 && connected,
  });
}
