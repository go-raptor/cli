package controllers_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestGreet(t *testing.T) {
	t.Run("returns greeting with empty greetings list", func(t *testing.T) {
		rec := app.TestGet("/api/v1/hello")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp["message"] != "Hello, World!" {
			t.Errorf("expected message 'Hello, World!', got %v", resp["message"])
		}

		greetings, ok := resp["greetings"].([]any)
		if !ok {
			t.Fatalf("expected greetings to be an array, got %T", resp["greetings"])
		}
		if len(greetings) != 0 {
			t.Errorf("expected empty greetings, got %d items", len(greetings))
		}
	})

	t.Run("adds greeting and returns it in list", func(t *testing.T) {
		body := strings.NewReader(`{"greeting": "Hi there!"}`)
		rec := app.TestPost("/api/v1/hello", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rec.Code)
		}

		rec = app.TestGet("/api/v1/hello")
		var resp map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		greetings, ok := resp["greetings"].([]any)
		if !ok {
			t.Fatalf("expected greetings to be an array, got %T", resp["greetings"])
		}
		if len(greetings) == 0 {
			t.Fatal("expected greetings to contain an entry")
		}
		if greetings[len(greetings)-1] != "Hi there!" {
			t.Errorf("expected last greeting to be 'Hi there!', got %v", greetings[len(greetings)-1])
		}
	})
}

func TestAddGreetingInvalidBody(t *testing.T) {
	body := strings.NewReader(`invalid json`)
	rec := app.TestPost("/api/v1/hello", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	rec := app.TestGet("/api/v1/nonexistent")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}
