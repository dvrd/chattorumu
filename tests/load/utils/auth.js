import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://chat-server:8080';

/**
 * Register a new user
 * @param {string} username - Unique username
 * @param {string} email - User email
 * @param {string} password - User password
 * @returns {boolean} Success status
 */
export function register(username, email, password) {
  const payload = JSON.stringify({
    username: username,
    email: email,
    password: password,
  });

  const res = http.post(`${BASE_URL}/api/v1/auth/register`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  const success = check(res, {
    'registration status is 201': (r) => r.status === 201,
  });

  if (!success) {
    console.error(`Registration failed for ${username}: ${res.status} - ${res.body}`);
  }

  return success;
}

/**
 * Login and get session token
 * @param {string} username - Username
 * @param {string} password - Password
 * @returns {string|null} Session token or null if failed
 */
export function login(username, password) {
  const payload = JSON.stringify({
    username: username,
    password: password,
  });

  const res = http.post(`${BASE_URL}/api/v1/auth/login`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  const success = check(res, {
    'login status is 200': (r) => r.status === 200,
    'login returns token': (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.session_token !== undefined;
      } catch (e) {
        return false;
      }
    },
  });

  if (!success) {
    console.error(`Login failed for ${username}: ${res.status} - ${res.body}`);
    return null;
  }

  try {
    const body = JSON.parse(res.body);
    return body.session_token;
  } catch (e) {
    console.error(`Failed to parse login response: ${e}`);
    return null;
  }
}

/**
 * Register and login a user in one call
 * @param {string} username - Unique username
 * @param {string} password - Password
 * @returns {string|null} Session token or null if failed
 */
export function registerAndLogin(username, password = 'testpass123') {
  const email = `${username}@loadtest.local`;

  register(username, email, password);
  sleep(0.5); // Small delay between register and login

  return login(username, password);
}
