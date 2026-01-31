//go:build e2e
// +build e2e

package e2e

import (
	"net/http"
	"testing"
)

func TestAuth_Register(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		client := NewTestClient(t)
		username := uniqueUsername("register")
		email := uniqueEmail("register")

		result, err := client.RegisterUser(username, email, "password123")
		assertNoError(t, err, "register should succeed")

		assertEqual(t, result.Username, username, "username should match")
		assertEqual(t, result.Email, email, "email should match")
		if result.ID == "" {
			t.Error("user ID should not be empty")
		}
	})

	t.Run("duplicate username rejected", func(t *testing.T) {
		client := NewTestClient(t)
		username := uniqueUsername("duplicate")
		email1 := uniqueEmail("duplicate1")
		email2 := uniqueEmail("duplicate2")

		// First registration should succeed
		_, err := client.RegisterUser(username, email1, "password123")
		assertNoError(t, err, "first registration should succeed")

		// Second registration with same username should fail
		_, err = client.RegisterUser(username, email2, "password123")
		if err == nil {
			t.Error("duplicate username registration should fail")
		}
	})

	t.Run("duplicate email rejected", func(t *testing.T) {
		client := NewTestClient(t)
		username1 := uniqueUsername("email1")
		username2 := uniqueUsername("email2")
		email := uniqueEmail("duplicate_email")

		// First registration should succeed
		_, err := client.RegisterUser(username1, email, "password123")
		assertNoError(t, err, "first registration should succeed")

		// Second registration with same email should fail
		_, err = client.RegisterUser(username2, email, "password123")
		if err == nil {
			t.Error("duplicate email registration should fail")
		}
	})

	t.Run("invalid email rejected", func(t *testing.T) {
		client := NewTestClient(t)
		username := uniqueUsername("invalid_email")

		_, err := client.RegisterUser(username, "not-an-email", "password123")
		if err == nil {
			t.Error("invalid email registration should fail")
		}
	})

	t.Run("short username rejected", func(t *testing.T) {
		client := NewTestClient(t)
		email := uniqueEmail("short_user")

		_, err := client.RegisterUser("ab", email, "password123")
		if err == nil {
			t.Error("short username registration should fail")
		}
	})

	t.Run("empty password rejected", func(t *testing.T) {
		client := NewTestClient(t)
		username := uniqueUsername("no_pass")
		email := uniqueEmail("no_pass")

		_, err := client.RegisterUser(username, email, "")
		if err == nil {
			t.Error("empty password registration should fail")
		}
	})
}

func TestAuth_Login(t *testing.T) {
	t.Run("successful login", func(t *testing.T) {
		client := NewTestClient(t)
		username := uniqueUsername("login")
		email := uniqueEmail("login")

		// Register first
		_, err := client.RegisterUser(username, email, "password123")
		assertNoError(t, err, "registration should succeed")

		// Login
		result, err := client.LoginUser(username, "password123")
		assertNoError(t, err, "login should succeed")

		if !result.Success {
			t.Error("login success should be true")
		}
		assertEqual(t, result.User.Username, username, "username should match")
		if result.SessionToken == "" {
			t.Error("session token should not be empty")
		}
	})

	t.Run("wrong password rejected", func(t *testing.T) {
		client := NewTestClient(t)
		username := uniqueUsername("wrong_pass")
		email := uniqueEmail("wrong_pass")

		// Register first
		_, err := client.RegisterUser(username, email, "password123")
		assertNoError(t, err, "registration should succeed")

		// Login with wrong password
		_, err = client.LoginUser(username, "wrongpassword")
		if err == nil {
			t.Error("login with wrong password should fail")
		}
	})

	t.Run("non-existent user rejected", func(t *testing.T) {
		client := NewTestClient(t)

		_, err := client.LoginUser("nonexistent_user_12345", "password123")
		if err == nil {
			t.Error("login with non-existent user should fail")
		}
	})

	t.Run("session cookie is set", func(t *testing.T) {
		client := NewTestClient(t)
		username := uniqueUsername("cookie")
		email := uniqueEmail("cookie")

		// Register and login
		_, err := client.RegisterUser(username, email, "password123")
		assertNoError(t, err, "registration should succeed")

		_, err = client.LoginUser(username, "password123")
		assertNoError(t, err, "login should succeed")

		// Check that we can access protected endpoint
		me, err := client.GetMe()
		assertNoError(t, err, "should be able to get current user")
		assertEqual(t, me.Username, username, "username should match")
	})
}

func TestAuth_Me(t *testing.T) {
	t.Run("returns current user", func(t *testing.T) {
		client := setupTestUser(t, "me")

		me, err := client.GetMe()
		assertNoError(t, err, "get me should succeed")

		assertEqual(t, me.Username, client.username, "username should match")
		assertEqual(t, me.ID, client.userID, "user ID should match")
	})

	t.Run("unauthorized without session", func(t *testing.T) {
		client := NewTestClient(t)

		resp, err := client.Get(baseURL + "/api/v1/auth/me")
		assertNoError(t, err, "request should not error")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})
}

func TestAuth_Logout(t *testing.T) {
	t.Run("successful logout", func(t *testing.T) {
		client := setupTestUser(t, "logout")

		// Should be able to access protected endpoint before logout
		_, err := client.GetMe()
		assertNoError(t, err, "should be able to get me before logout")

		// Logout
		err = client.Logout()
		assertNoError(t, err, "logout should succeed")

		// Should not be able to access protected endpoint after logout
		resp, err := client.Get(baseURL + "/api/v1/auth/me")
		assertNoError(t, err, "request should not error")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 after logout, got %d", resp.StatusCode)
		}
	})

	t.Run("logout without session returns error", func(t *testing.T) {
		client := NewTestClient(t)

		resp, err := client.PostJSON("/api/v1/auth/logout", nil)
		assertNoError(t, err, "request should not error")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 for logout without session, got %d", resp.StatusCode)
		}
	})
}

func TestAuth_SessionPersistence(t *testing.T) {
	t.Run("session persists across requests", func(t *testing.T) {
		client := setupTestUser(t, "persist")

		// Make multiple requests
		for i := 0; i < 3; i++ {
			me, err := client.GetMe()
			assertNoError(t, err, "get me should succeed")
			assertEqual(t, me.Username, client.username, "username should match")
		}
	})

	t.Run("different clients have independent sessions", func(t *testing.T) {
		client1 := setupTestUser(t, "user1")
		client2 := setupTestUser(t, "user2")

		me1, err := client1.GetMe()
		assertNoError(t, err, "client1 get me should succeed")

		me2, err := client2.GetMe()
		assertNoError(t, err, "client2 get me should succeed")

		if me1.Username == me2.Username {
			t.Error("different clients should have different users")
		}
	})
}
