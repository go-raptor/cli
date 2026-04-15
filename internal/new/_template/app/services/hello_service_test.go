package services_test

import (
	"testing"

	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/template/app/services"
)

func TestGreeting(t *testing.T) {
	s := raptor.GetService[services.HelloService](app)
	if got := s.Greeting(); got != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got '%s'", got)
	}
}

func TestGreetings(t *testing.T) {
	s := raptor.GetService[services.HelloService](app)

	t.Run("empty initially", func(t *testing.T) {
		greetings := s.Greetings()
		if len(greetings) != 0 {
			t.Errorf("expected empty greetings, got %d items", len(greetings))
		}
	})

	t.Run("after adding", func(t *testing.T) {
		s.AddGreeting("Hey!")
		s.AddGreeting("Howdy!")

		greetings := s.Greetings()
		if len(greetings) != 2 {
			t.Fatalf("expected 2 greetings, got %d", len(greetings))
		}
		if greetings[0] != "Hey!" {
			t.Errorf("expected first greeting 'Hey!', got '%s'", greetings[0])
		}
		if greetings[1] != "Howdy!" {
			t.Errorf("expected second greeting 'Howdy!', got '%s'", greetings[1])
		}
	})
}
