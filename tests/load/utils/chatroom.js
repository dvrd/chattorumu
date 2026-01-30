import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://chat-server:8080';

/**
 * Create a new chatroom
 * @param {string} name - Chatroom name
 * @param {string} token - Auth token
 * @returns {string|null} Chatroom ID or null if failed
 */
export function createChatroom(name, token) {
  const payload = JSON.stringify({
    name: name,
  });

  const res = http.post(`${BASE_URL}/api/v1/chatrooms`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  });

  const success = check(res, {
    'chatroom created': (r) => r.status === 201,
  });

  if (!success) {
    console.error(`Failed to create chatroom ${name}: ${res.status} - ${res.body}`);
    return null;
  }

  try {
    const body = JSON.parse(res.body);
    return body.id;
  } catch (e) {
    console.error(`Failed to parse chatroom creation response: ${e}`);
    return null;
  }
}

/**
 * Join a chatroom
 * @param {string} chatroomId - Chatroom ID
 * @param {string} token - Auth token
 * @returns {boolean} Success status
 */
export function joinChatroom(chatroomId, token) {
  const res = http.post(`${BASE_URL}/api/v1/chatrooms/${chatroomId}/join`, null, {
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  });

  const success = check(res, {
    'joined chatroom': (r) => r.status === 200,
  });

  if (!success) {
    console.error(`Failed to join chatroom ${chatroomId}: ${res.status} - ${res.body}`);
  }

  return success;
}

/**
 * Get list of chatrooms
 * @param {string} token - Auth token
 * @returns {Array|null} Array of chatrooms or null if failed
 */
export function listChatrooms(token) {
  const res = http.get(`${BASE_URL}/api/v1/chatrooms`, {
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  });

  const success = check(res, {
    'chatrooms listed': (r) => r.status === 200,
  });

  if (!success) {
    console.error(`Failed to list chatrooms: ${res.status} - ${res.body}`);
    return null;
  }

  try {
    const body = JSON.parse(res.body);
    return body.chatrooms || [];
  } catch (e) {
    console.error(`Failed to parse chatrooms list: ${e}`);
    return null;
  }
}
