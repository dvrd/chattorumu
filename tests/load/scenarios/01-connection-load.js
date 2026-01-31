/**
 * Scenario 1: Basic Connection Load (Baseline)
 *
 * Goal: Measure maximum concurrent WebSocket connections
 *
 * Test Flow:
 * - Ramp up users: 0 → 1000 over 5 minutes
 * - Hold: 1000 users for 10 minutes
 * - Ramp down: 1000 → 0 over 2 minutes
 *
 * Success Criteria:
 * - Connection success rate > 99%
 * - Average latency < 100ms
 * - No memory leaks (check metrics)
 * - Graceful degradation
 */

import { sleep } from 'k6';
import { registerAndLogin } from '../utils/auth.js';
import { createChatroom, joinChatroom } from '../utils/chatroom.js';
import { connectAndHold } from '../utils/websocket.js';

export const options = {
  stages: [
    { duration: '5m', target: 1000 },  // Ramp up to 1000 users
    { duration: '10m', target: 1000 }, // Hold at 1000 users
    { duration: '2m', target: 0 },     // Ramp down to 0
  ],
  thresholds: {
    // WebSocket connection should be fast
    'ws_connecting': ['p(95)<1000', 'p(99)<2000'],

    // Connection success rate should be high
    'websocket_connection_success': ['rate>0.99'],

    // Session duration should be stable
    'ws_session_duration': ['p(95)<605000'], // ~10min in ms

    // Error rate should be low
    'websocket_errors': ['count<10'],

    // HTTP checks should pass
    'checks': ['rate>0.95'],
  },
  // Set a tag for this scenario
  tags: {
    scenario: 'connection-load',
    test_type: 'baseline',
  },
};

// Use pre-created chatroom if available
const CHATROOM_ID = __ENV.CHATROOM_ID || 'load-test-room';

export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 1: Basic Connection Load');
  console.log('='.repeat(60));
  console.log('Target: 1000 concurrent connections');
  console.log('Duration: 17 minutes (5min ramp + 10min hold + 2min down)');
  console.log('');

  // Create a test user and chatroom for setup
  const adminUsername = `load_test_admin_${Date.now()}`;
  const adminToken = registerAndLogin(adminUsername);
  if (!adminToken) {
    throw new Error('Failed to create admin user for setup');
  }

  // Try to create the default chatroom (might already exist)
  const chatroomId = createChatroom('Load Test Room', adminToken);

  return {
    chatroomId: chatroomId || CHATROOM_ID,
  };
}

export default function(data) {
  const timestamp = Date.now();
  const userId = `user_${__VU}_${__ITER}_${timestamp}`;

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

  // Connect to WebSocket and hold the connection
  // Connection will be held for the duration of the stage
  connectAndHold(data.chatroomId, token, 600); // Hold for 10 minutes

  // Small sleep to prevent tight loop
  sleep(1);
}

export function teardown(data) {
  console.log('');
  console.log('='.repeat(60));
  console.log('Test completed. Check Grafana for results:');
  console.log('  - WebSocket Connections Active');
  console.log('  - Memory Usage');
  console.log('  - Goroutine Count');
  console.log('  - Error Rate');
  console.log('='.repeat(60));
}
