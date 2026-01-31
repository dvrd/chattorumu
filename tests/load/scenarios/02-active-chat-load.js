/**
 * Scenario 2: Active Chat Load (Realistic)
 *
 * Goal: Simulate real users chatting
 *
 * Test Flow:
 * - 500 concurrent users
 * - Each sends 1-5 messages per minute
 * - Random distribution across 10 chatrooms
 * - Mix of text messages (80%) and stock commands (20%)
 *
 * Success Criteria:
 * - Message delivery rate > 99.9%
 * - P95 latency < 200ms
 * - RabbitMQ queue not backing up
 * - DB connection pool stable
 */

import { sleep } from 'k6';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { registerAndLogin } from '../utils/auth.js';
import { createChatroom, joinChatroom } from '../utils/chatroom.js';
import { chatSession } from '../utils/websocket.js';

export const options = {
  stages: [
    { duration: '2m', target: 500 },   // Ramp up to 500 users
    { duration: '10m', target: 500 },  // Hold at 500 users
    { duration: '1m', target: 0 },     // Ramp down to 0
  ],
  thresholds: {
    // Message latency should be low
    'chat_message_latency_ms': ['p(95)<200', 'p(99)<500'],

    // Stock command latency can be higher (API call)
    'stock_command_latency_ms': ['p(95)<2000', 'p(99)<5000'],

    // Message delivery rate should be very high
    'message_delivery_success': ['rate>0.999'],

    // Connection success
    'websocket_connection_success': ['rate>0.99'],

    // WebSocket errors should be minimal
    'websocket_errors': ['count<50'],

    // HTTP checks
    'checks': ['rate>0.95'],

    // Messages should be flowing
    'chat_messages_sent': ['count>10000'],
    'chat_messages_received': ['count>10000'],
  },
  tags: {
    scenario: 'active-chat',
    test_type: 'realistic',
  },
};

const NUM_CHATROOMS = 10;

export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 2: Active Chat Load (Realistic)');
  console.log('='.repeat(60));
  console.log('Target: 500 users across 10 chatrooms');
  console.log('Activity: 1-5 messages/min per user');
  console.log('Mix: 80% chat messages, 20% stock commands');
  console.log('Duration: 13 minutes');
  console.log('');

  // Create admin user
  const adminToken = registerAndLogin('chat_test_admin');
  if (!adminToken) {
    throw new Error('Failed to create admin user');
  }

  // Create multiple chatrooms
  const chatrooms = [];
  for (let i = 1; i <= NUM_CHATROOMS; i++) {
    const roomName = `Active Chat Room ${i}`;
    const roomId = createChatroom(roomName, adminToken);
    if (roomId) {
      chatrooms.push(roomId);
      console.log(`Created chatroom: ${roomName} (${roomId})`);
    }
  }

  if (chatrooms.length === 0) {
    throw new Error('Failed to create any chatrooms');
  }

  console.log(`Successfully created ${chatrooms.length} chatrooms`);
  console.log('');

  return { chatrooms };
}

export default function(data) {
  const userId = `chat_user_${__VU}_${__ITER}`;

  // Register and login
  const token = registerAndLogin(userId);
  if (!token) {
    console.error(`Failed to authenticate user ${userId}`);
    return;
  }

  // Pick a random chatroom (some will be more popular than others)
  const chatroomId = data.chatrooms[randomIntBetween(0, data.chatrooms.length - 1)];

  // Join the chatroom
  const joined = joinChatroom(chatroomId, token);
  if (!joined) {
    console.error(`Failed to join chatroom for user ${userId}`);
    return;
  }

  // Calculate message interval (1-5 messages per minute = 12-60 seconds between messages)
  const messagesPerMinute = randomIntBetween(1, 5);
  const messageInterval = (60 / messagesPerMinute) * 1000; // Convert to milliseconds

  // 20% of users will also send stock commands
  const sendsStockCommands = Math.random() < 0.2;
  const stockCommandInterval = sendsStockCommands ? 60000 : 0; // Every minute if enabled

  // Start chat session
  chatSession(chatroomId, token, {
    duration: 600, // 10 minutes
    messageInterval: messageInterval,
    stockCommandInterval: stockCommandInterval,
  });

  // Small sleep between iterations
  sleep(1);
}

export function teardown(data) {
  console.log('');
  console.log('='.repeat(60));
  console.log('Active Chat Load Test Completed');
  console.log('');
  console.log('Check Grafana dashboards for:');
  console.log('  - Message throughput (messages/min)');
  console.log('  - Message latency (P95, P99)');
  console.log('  - Top active chatrooms');
  console.log('  - Stock command processing time');
  console.log('  - RabbitMQ queue depth');
  console.log('  - Database connection pool usage');
  console.log('='.repeat(60));
}
