/**
 * Scenario 6: Multi-Room Load Distribution
 *
 * Goal: Test hub broadcast efficiency with uneven distribution
 *
 * Test Flow:
 * - 1000 users across 50 chatrooms
 * - Uneven distribution (some rooms with 100 users, some with 2)
 * - High message volume in large rooms
 *
 * Success Criteria:
 * - Large rooms don't affect small rooms
 * - Broadcast doesn't block hub
 * - Fair message delivery across rooms
 * - No broadcast channel overflow
 */

import { sleep } from 'k6';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { registerAndLogin } from '../utils/auth.js';
import { createChatroom, joinChatroom } from '../utils/chatroom.js';
import { chatSession } from '../utils/websocket.js';

export const options = {
  stages: [
    { duration: '3m', target: 1000 },  // Ramp up to 1000 users
    { duration: '10m', target: 1000 }, // Hold at 1000 users
    { duration: '2m', target: 0 },     // Ramp down
  ],
  thresholds: {
    // Message latency should be consistent across rooms
    'chat_message_latency_ms': ['p(95)<300', 'p(99)<1000'],

    // High message throughput expected
    'chat_messages_sent': ['count>50000'],
    'chat_messages_received': ['count>50000'],

    // Message delivery should be reliable
    'message_delivery_success': ['rate>0.99'],

    // Connection success
    'websocket_connection_success': ['rate>0.99'],

    // Error rate
    'websocket_errors': ['count<50'],

    // HTTP checks
    'checks': ['rate>0.95'],
  },
  tags: {
    scenario: 'multi-room-load',
    test_type: 'distribution',
  },
};

const NUM_CHATROOMS = 50;

// Weighted room selection - simulates Pareto distribution
// 20% of rooms will have 80% of users (realistic chat pattern)
function selectRoomWeighted(chatrooms) {
  const rand = Math.random();

  if (rand < 0.80) {
    // 80% of users join the top 20% of rooms (popular rooms)
    const popularRoomCount = Math.floor(NUM_CHATROOMS * 0.2);
    return chatrooms[randomIntBetween(0, popularRoomCount - 1)];
  } else {
    // 20% of users distributed across remaining 80% of rooms
    const quietRoomStart = Math.floor(NUM_CHATROOMS * 0.2);
    return chatrooms[randomIntBetween(quietRoomStart, chatrooms.length - 1)];
  }
}

export function setup() {
  console.log('='.repeat(60));
  console.log('Scenario 6: Multi-Room Load Distribution');
  console.log('='.repeat(60));
  console.log('');
  console.log('Test Configuration:');
  console.log(`  Total Users: 1000`);
  console.log(`  Total Chatrooms: ${NUM_CHATROOMS}`);
  console.log(`  Distribution: Pareto (80/20 rule)`);
  console.log(`    - Top 10 rooms: ~800 users (80%)`);
  console.log(`    - Remaining 40 rooms: ~200 users (20%)`);
  console.log('');
  console.log('What This Tests:');
  console.log('  1. Broadcast Efficiency:');
  console.log('     - Large rooms (100 users) broadcasting messages');
  console.log('     - Should not block small rooms');
  console.log('');
  console.log('  2. Hub Performance:');
  console.log('     - Can hub.Run() handle multiple busy rooms?');
  console.log('     - Broadcast channel buffer sufficient?');
  console.log('     - No goroutine blocking?');
  console.log('');
  console.log('  3. Isolation:');
  console.log('     - Message in Room A should not delay Room B');
  console.log('     - Slow client in one room should not affect others');
  console.log('');
  console.log('  4. Fairness:');
  console.log('     - All rooms get equal priority');
  console.log('     - No starvation of small rooms');
  console.log('');

  // Create admin
  const adminToken = registerAndLogin('multiroom-admin');
  if (!adminToken) {
    throw new Error('Failed to create admin user');
  }

  // Create 50 chatrooms
  const chatrooms = [];
  console.log('Creating chatrooms...');

  for (let i = 1; i <= NUM_CHATROOMS; i++) {
    const roomName = `Room-${String(i).padStart(3, '0')}`;
    const roomId = createChatroom(roomName, adminToken);
    if (roomId) {
      chatrooms.push({
        id: roomId,
        name: roomName,
        index: i,
      });
    }

    // Progress indicator
    if (i % 10 === 0) {
      console.log(`  Created ${i}/${NUM_CHATROOMS} chatrooms...`);
    }
  }

  if (chatrooms.length < NUM_CHATROOMS) {
    console.warn(`⚠️  Only created ${chatrooms.length}/${NUM_CHATROOMS} chatrooms`);
  }

  console.log(`✅ Created ${chatrooms.length} chatrooms`);
  console.log('');
  console.log('Distribution will follow Pareto principle:');
  console.log(`  - Popular rooms (1-${Math.floor(NUM_CHATROOMS * 0.2)}): ~80% of users`);
  console.log(`  - Regular rooms (${Math.floor(NUM_CHATROOMS * 0.2) + 1}-${NUM_CHATROOMS}): ~20% of users`);
  console.log('');
  console.log('Starting test...');
  console.log('');

  return { chatrooms };
}

export default function(data) {
  const userId = `room_user_${__VU}_${__ITER}`;

  // Register and login
  const token = registerAndLogin(userId);
  if (!token) {
    console.error(`Failed to authenticate user ${userId}`);
    return;
  }

  // Select room using weighted distribution (Pareto)
  const selectedRoom = selectRoomWeighted(data.chatrooms);

  // Join the selected chatroom
  const joined = joinChatroom(selectedRoom.id, token);
  if (!joined) {
    console.error(`Failed to join chatroom ${selectedRoom.name}`);
    return;
  }

  // Activity level varies by room popularity
  // Popular rooms: higher activity
  // Quiet rooms: lower activity
  const isPopularRoom = selectedRoom.index <= Math.floor(NUM_CHATROOMS * 0.2);
  const messagesPerMinute = isPopularRoom
    ? randomIntBetween(5, 15)  // Popular rooms: very active
    : randomIntBetween(1, 5);  // Quiet rooms: less active

  const messageInterval = (60 / messagesPerMinute) * 1000;

  // Start chat session
  chatSession(selectedRoom.id, token, {
    duration: 600, // 10 minutes
    messageInterval: messageInterval,
    stockCommandInterval: 0, // No stock commands in this test
  });

  sleep(1);
}

export function teardown(data) {
  console.log('');
  console.log('='.repeat(60));
  console.log('Multi-Room Load Distribution Test Completed');
  console.log('='.repeat(60));
  console.log('');
  console.log('Analysis Guide:');
  console.log('');
  console.log('1. Check "Top 5 Chatrooms" Panel:');
  console.log('   - Should show uneven distribution');
  console.log('   - Top rooms should have much higher message counts');
  console.log('   - This is expected (Pareto distribution)');
  console.log('');
  console.log('2. Message Latency Consistency:');
  console.log('   - Compare P95 latency across popular vs quiet rooms');
  console.log('   - Should be similar (within 2x)');
  console.log('   - Large difference = isolation problem');
  console.log('');
  console.log('3. Hub Performance:');
  console.log('   - Check goroutine count during peak');
  console.log('   - Should be stable (not growing)');
  console.log('   - Monitor CPU usage of chat-server');
  console.log('');
  console.log('4. Broadcast Channel:');
  console.log('   - If you see "broadcast channel full" errors:');
  console.log('     → Increase buffer size in hub.go');
  console.log('     → Current: 256, try 1024 or 4096');
  console.log('');
  console.log('5. Message Delivery Rate:');
  console.log('   - Should be >99% across all rooms');
  console.log('   - Lower rate in specific rooms = problem');
  console.log('');
  console.log('Expected Behavior:');
  console.log('  ✅ All rooms process messages independently');
  console.log('  ✅ Popular rooms handle high volume without blocking');
  console.log('  ✅ Quiet rooms still get fair treatment');
  console.log('  ✅ No broadcast channel overflow');
  console.log('  ✅ Consistent latency across room sizes');
  console.log('');
  console.log('Red Flags:');
  console.log('  ❌ Latency increases with room size');
  console.log('  ❌ Small rooms delayed when big rooms are busy');
  console.log('  ❌ Broadcast channel errors');
  console.log('  ❌ Goroutine count growing unbounded');
  console.log('  ❌ Message delivery rate varying by room');
  console.log('='.repeat(60));
}
