/**
 * Scenario 4: Spike Test (Resilience)
 *
 * Goal: Test sudden traffic surge and recovery
 *
 * Test Flow:
 * - Baseline: 100 users
 * - Spike: 0 → 1000 users in 30 seconds
 * - Hold: 1000 users for 5 minutes
 * - Drop: 1000 → 100 in 30 seconds
 * - Recovery: 100 users for 2 minutes
 *
 * Success Criteria:
 * - No crashes during spike
 * - Rate limiting works correctly
 * - System recovers after spike
 * - Error rate < 1%
 */

import { sleep } from 'k6';
import { registerAndLogin } from '../utils/auth.js';
import { createChatroom, joinChatroom } from '../utils/chatroom.js';
import { chatSession } from '../utils/websocket.js';

export const options = {
  stages: [
    { duration: '2m', target: 100 },   // Baseline: 100 users
    { duration: '30s', target: 1000 }, // SPIKE: 100 → 1000 in 30 seconds
    { duration: '5m', target: 1000 },  // Hold spike
    { duration: '30s', target: 100 },  // Drop: 1000 → 100 in 30 seconds
    { duration: '2m', target: 100 },   // Recovery: observe system behavior
  ],
  thresholds: {
    // System should handle spike gracefully
    'websocket_connection_success': ['rate>0.95'], // 95% during spike is acceptable

    // Latency may increase during spike but should recover
    'chat_message_latency_ms': ['p(99)<1000'],

    // Error rate should be low even during spike
    'websocket_errors': ['count<100'],

    // Connection time during spike
    'ws_connecting': ['p(99)<5000'], // May be slower during spike

    // HTTP checks
    'checks': ['rate>0.90'], // Lower threshold for spike test

    // Messages should still flow
    'message_delivery_success': ['rate>0.90'],
  },
  tags: {
    scenario: 'spike-test',
    test_type: 'resilience',
  },
};

export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 4: Spike Test (Resilience)');
  console.log('='.repeat(60));
  console.log('');
  console.log('Test Pattern:');
  console.log('  Phase 1 (2m):    Baseline at 100 users');
  console.log('  Phase 2 (30s):   SPIKE to 1000 users  <-- Critical!');
  console.log('  Phase 3 (5m):    Hold at 1000 users');
  console.log('  Phase 4 (30s):   Drop to 100 users');
  console.log('  Phase 5 (2m):    Recovery observation');
  console.log('');
  console.log('What we are testing:');
  console.log('  - System behavior under sudden load');
  console.log('  - Connection queue handling');
  console.log('  - Resource allocation under stress');
  console.log('  - Recovery and stabilization');
  console.log('  - No cascading failures');
  console.log('');

  // Create admin and chatroom
  const adminToken = registerAndLogin('spike-test-admin');
  if (!adminToken) {
    throw new Error('Failed to create admin user');
  }

  const chatroomId = createChatroom('Spike Test Room', adminToken);
  if (!chatroomId) {
    throw new Error('Failed to create test chatroom');
  }

  console.log(`Test chatroom created: ${chatroomId}`);
  console.log('');
  console.log('Starting test...');
  console.log('');

  return { chatroomId };
}

export default function(data) {
  const userId = `spike_user_${__VU}_${__ITER}`;

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

  // Start moderate chat activity
  chatSession(data.chatroomId, token, {
    duration: 300, // 5 minutes
    messageInterval: 15000, // Message every 15 seconds
    stockCommandInterval: 0, // No stock commands in spike test
  });

  sleep(1);
}

export function teardown(data) {
  console.log('');
  console.log('='.repeat(60));
  console.log('Spike Test Completed');
  console.log('='.repeat(60));
  console.log('');
  console.log('Critical Analysis Points:');
  console.log('');
  console.log('1. DURING SPIKE (30 second window):');
  console.log('   - Did any services crash?');
  console.log('   - Were new connections accepted?');
  console.log('   - Did error rate spike above 5%?');
  console.log('   - Did response times increase significantly?');
  console.log('');
  console.log('2. AT PEAK (1000 users):');
  console.log('   - System stability maintained?');
  console.log('   - Memory usage acceptable?');
  console.log('   - No goroutine leaks?');
  console.log('   - Message delivery still functioning?');
  console.log('');
  console.log('3. DURING DROP (30 second window):');
  console.log('   - Graceful disconnections?');
  console.log('   - Resources released properly?');
  console.log('   - No errors during cleanup?');
  console.log('');
  console.log('4. RECOVERY PHASE (2 minutes):');
  console.log('   - System returned to baseline performance?');
  console.log('   - No lingering issues?');
  console.log('   - Memory returned to normal?');
  console.log('   - Response times back to baseline?');
  console.log('');
  console.log('Grafana Dashboard Panels to Check:');
  console.log('  - WebSocket Connections Active (should show spike pattern)');
  console.log('  - Memory Usage (should stabilize after recovery)');
  console.log('  - Goroutines (should return to baseline)');
  console.log('  - Error Rate (should be minimal)');
  console.log('  - Message Latency (should recover)');
  console.log('='.repeat(60));
}
