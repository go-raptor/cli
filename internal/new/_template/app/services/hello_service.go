package services

import (
	"github.com/go-raptor/raptor/v4"
)

type HelloService struct {
	raptor.Service

	greetings []string
}

func (s *HelloService) Setup() error {
	s.greetings = []string{}
	return nil
}

func (s *HelloService) Cleanup() error {
	return nil
}

func (s *HelloService) Greeting() string {
	return "Hello, World!"
}

func (s *HelloService) Greetings() []string {
	return s.greetings
}

func (s *HelloService) AddGreeting(greeting string) {
	s.greetings = append(s.greetings, greeting)
}
