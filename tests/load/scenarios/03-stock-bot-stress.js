/**
 * Scenario 3: Stock Bot Stress (Command Heavy)
 *
 * Goal: Test stock command processing under heavy load
 *
 * Test Flow:
 * - 100 concurrent users
 * - Each sends /stock= command every 10 seconds
 * - Monitor RabbitMQ message flow
 * - Check stock bot response time
 *
 * Success Criteria:
 * - Command processing time < 2s (P95)
 * - RabbitMQ not dropping messages
 * - Stock bot doesn't crash
 * - Database writes stable
 */

import { sleep } from 'k6';
import { registerAndLogin } from '../utils/auth.js';
import { createChatroom, joinChatroom } from '../utils/chatroom.js';
import { chatSession } from '../utils/websocket.js';

export const options = {
  stages: [
    { duration: '1m', target: 100 },   // Ramp up to 100 users
    { duration: '5m', target: 100 },   // Hold at 100 users (sustained load)
    { duration: '30s', target: 0 },    // Ramp down
  ],
  thresholds: {
    // Stock command latency (includes API call to Stooq + RabbitMQ + DB)
    'stock_command_latency_ms': [
      'p(95)<2000',  // 95% under 2 seconds
      'p(99)<5000',  // 99% under 5 seconds
    ],

    // All stock commands should get responses
    'stock_commands_sent': ['count>0'],
    'stock_responses_received': ['count>0'],

    // Calculate response rate
    'message_delivery_success': ['rate>0.95'],

    // Connection success
    'websocket_connection_success': ['rate>0.99'],

    // Error rate
    'websocket_errors': ['count<20'],

    // HTTP checks
    'checks': ['rate>0.95'],
  },
  tags: {
    scenario: 'stock-bot-stress',
    test_type: 'stress',
  },
};

export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 3: Stock Bot Stress Test');
  console.log('='.repeat(60));
  console.log('Target: 100 users sending stock commands');
  console.log('Frequency: 1 command per user every 10 seconds');
  console.log('Rate: ~600 commands/minute (~10/second)');
  console.log('Duration: 6.5 minutes');
  console.log('');
  console.log('This test stresses:');
  console.log('  - Stock bot command processing');
  console.log('  - RabbitMQ message throughput');
  console.log('  - Stooq API rate limits');
  console.log('  - Database write performance');
  console.log('');

  // Create admin user and chatroom
  const adminToken = registerAndLogin(`stock_test_admin_${Date.now()}`);
  if (!adminToken) {
    throw new Error('Failed to create admin user');
  }

  const chatroomId = createChatroom('Stock Bot Test Room', adminToken);
  if (!chatroomId) {
    throw new Error('Failed to create test chatroom');
  }

  console.log(`Created test chatroom: ${chatroomId}`);
  console.log('');

  return { chatroomId };
}

export default function(data) {
  const timestamp = Date.now();
  const userId = `stock_tester_${__VU}_${__ITER}_${timestamp}`;

  // Register and login
  const token = registerAndLogin(userId);
  if (!token) {
    console.error(`Failed to authenticate user ${userId}`);
    return;
  }

  // Join the chatroom
  const joined = joinChatroom(data.chatroomId, token);
  if (!joined) {
    console.error(`Failed to join chatroom for user ${userId}`);
    return;
  }

  // Start chat session with high frequency of stock commands
  chatSession(data.chatroomId, token, {
    duration: 300, // 5 minutes
    messageInterval: 30000, // Regular message every 30 seconds
    stockCommandInterval: 10000, // Stock command every 10 seconds
    onMessage: (msg) => {
      // Log stock bot responses for debugging
      if (msg.type === 'stock_response' || (msg.content && msg.content.includes('quote'))) {
        console.log(`Received stock response: ${msg.content.substring(0, 50)}...`);
      }
    },
  });

  sleep(1);
}

export function teardown(data) {
  console.log('');
  console.log('='.repeat(60));
  console.log('Stock Bot Stress Test Completed');
  console.log('');
  console.log('Key Metrics to Review:');
  console.log('  1. Stock command latency (P95, P99)');
  console.log('  2. Stock commands sent vs responses received');
  console.log('  3. RabbitMQ queue depth during test');
  console.log('  4. Stock bot CPU and memory usage');
  console.log('  5. Database write throughput');
  console.log('  6. Stooq API error rate (if any)');
  console.log('');
  console.log('Expected Results:');
  console.log('  - ~600 commands/minute processed');
  console.log('  - P95 latency < 2 seconds');
  console.log('  - No message drops');
  console.log('  - Stable memory usage');
  console.log('='.repeat(60));
}
