/**
 * Scenario 5: Soak Test (Stability)
 *
 * Goal: Detect memory leaks and resource exhaustion
 *
 * Test Flow:
 * - 300 concurrent users
 * - Continuous activity for 2+ hours (configurable)
 * - Monitor memory, goroutines, connections
 *
 * Success Criteria:
 * - No memory leaks (stable or bounded growth)
 * - Goroutine count stable
 * - DB connections stable
 * - Performance doesn't degrade over time
 *
 * Note: This is a long-running test. Use SHORT_SOAK env var for 15-minute test.
 */

import { sleep } from 'k6';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { registerAndLogin } from '../utils/auth.js';
import { createChatroom, joinChatroom } from '../utils/chatroom.js';
import { chatSession } from '../utils/websocket.js';

// Allow short soak test for quick validation
const SHORT_SOAK = __ENV.SHORT_SOAK === 'true';
const SOAK_DURATION = SHORT_SOAK ? '15m' : '2h';
const RAMP_UP = SHORT_SOAK ? '2m' : '5m';

export const options = {
  stages: [
    { duration: RAMP_UP, target: 300 },      // Ramp up
    { duration: SOAK_DURATION, target: 300 }, // Long soak period
    { duration: '2m', target: 0 },           // Ramp down
  ],
  thresholds: {
    // Performance should not degrade over time
    'chat_message_latency_ms': ['p(95)<300', 'p(99)<1000'],

    // Connection success should remain high throughout
    'websocket_connection_success': ['rate>0.99'],

    // Error rate should stay low
    'websocket_errors': ['count<100'],

    // Message delivery
    'message_delivery_success': ['rate>0.99'],

    // HTTP checks
    'checks': ['rate>0.95'],
  },
  tags: {
    scenario: 'soak-test',
    test_type: 'stability',
  },
};

const NUM_CHATROOMS = 5;

export function setup() {
  const testDuration = SHORT_SOAK ? '15 minutes' : '2+ hours';

  console.log('='.repeat(60));
  console.log('Scenario 5: Soak Test (Stability & Memory Leak Detection)');
  console.log('='.repeat(60));
  console.log('');
  console.log(`Test Configuration:`);
  console.log(`  Users: 300 concurrent`);
  console.log(`  Duration: ${testDuration}`);
  console.log(`  Chatrooms: ${NUM_CHATROOMS}`);
  console.log(`  Mode: ${SHORT_SOAK ? 'SHORT (validation)' : 'FULL (production)'}`);
  console.log('');
  console.log('What to Monitor:');
  console.log('  1. Memory Usage - Should be stable or show bounded growth');
  console.log('  2. Goroutine Count - Should not continuously increase');
  console.log('  3. DB Connection Pool - Should remain stable');
  console.log('  4. Message Latency - Should not increase over time');
  console.log('  5. CPU Usage - Should be steady');
  console.log('  6. WebSocket Connections - Should match user count');
  console.log('');
  console.log('Red Flags:');
  console.log('  ❌ Memory steadily increasing without bound');
  console.log('  ❌ Goroutine count growing continuously');
  console.log('  ❌ Performance degrading over time');
  console.log('  ❌ Connection leaks (connections > users)');
  console.log('  ❌ File descriptor exhaustion');
  console.log('');

  if (!SHORT_SOAK) {
    console.log('⚠️  FULL SOAK TEST - This will run for 2+ hours!');
    console.log('   Set SHORT_SOAK=true for 15-minute validation test');
    console.log('');
  }

  // Create admin and chatrooms
  const adminToken = registerAndLogin('soak_test_admin');
  if (!adminToken) {
    throw new Error('Failed to create admin user');
  }

  const chatrooms = [];
  for (let i = 1; i <= NUM_CHATROOMS; i++) {
    const roomId = createChatroom(`Soak Test Room ${i}`, adminToken);
    if (roomId) {
      chatrooms.push(roomId);
    }
  }

  if (chatrooms.length === 0) {
    throw new Error('Failed to create any chatrooms');
  }

  console.log(`Created ${chatrooms.length} test chatrooms`);
  console.log('');
  console.log('Test starting... Grab some coffee ☕');
  console.log('');

  return { chatrooms };
}

export default function(data) {
  const userId = `soak_user_${__VU}_${__ITER}`;

  // Register and login
  const token = registerAndLogin(userId);
  if (!token) {
    console.error(`Failed to authenticate user ${userId}`);
    return;
  }

  // Pick a random chatroom
  const chatroomId = data.chatrooms[randomIntBetween(0, data.chatrooms.length - 1)];

  // Join the chatroom
  const joined = joinChatroom(chatroomId, token);
  if (!joined) {
    console.error(`Failed to join chatroom for user ${userId}`);
    return;
  }

  // Realistic user behavior - varying activity levels
  const messagesPerMinute = randomIntBetween(2, 10); // 2-10 messages per minute
  const messageInterval = (60 / messagesPerMinute) * 1000;

  // 10% of users send stock commands
  const sendsStockCommands = Math.random() < 0.1;
  const stockCommandInterval = sendsStockCommands ? 120000 : 0; // Every 2 minutes

  // Long session duration for soak test
  const sessionDuration = SHORT_SOAK ? 900 : 7200; // 15 min or 2 hours

  // Start chat session
  chatSession(chatroomId, token, {
    duration: sessionDuration,
    messageInterval: messageInterval,
    stockCommandInterval: stockCommandInterval,
  });

  sleep(2);
}

export function teardown(data) {
  const testDuration = SHORT_SOAK ? '15 minutes' : '2+ hours';

  console.log('');
  console.log('='.repeat(60));
  console.log('Soak Test Completed');
  console.log('='.repeat(60));
  console.log('');
  console.log(`Test ran for: ${testDuration}`);
  console.log('');
  console.log('Analysis Checklist:');
  console.log('');
  console.log('□ 1. Memory Usage Analysis:');
  console.log('     - Open Grafana "Memory Usage" panel');
  console.log('     - Check if memory shows linear growth (BAD)');
  console.log('     - Or stable/sawtooth pattern (GOOD)');
  console.log('');
  console.log('□ 2. Goroutine Analysis:');
  console.log('     - Open Grafana "Goroutines" panel');
  console.log('     - Should be relatively stable (< 1000 for 300 users)');
  console.log('     - Continuous growth = goroutine leak');
  console.log('');
  console.log('□ 3. Performance Degradation:');
  console.log('     - Compare P95 latency at start vs end');
  console.log('     - Should be within 20% of each other');
  console.log('     - Significant increase = performance issue');
  console.log('');
  console.log('□ 4. Database Connections:');
  console.log('     - Check "Database Connection Pool" panel');
  console.log('     - Should be stable throughout');
  console.log('     - All connections in use = need larger pool');
  console.log('');
  console.log('□ 5. Error Rate Over Time:');
  console.log('     - Should remain consistently low');
  console.log('     - Increasing errors = system degradation');
  console.log('');
  console.log('□ 6. WebSocket Connections:');
  console.log('     - Should match user count (300)');
  console.log('     - Higher = connection leak');
  console.log('     - Lower = connections dropping');
  console.log('');
  console.log('If all checks pass: ✅ System is stable for production');
  console.log('If any fail: ⚠️  Investigate and fix before scaling');
  console.log('');
  console.log('='.repeat(60));
}
