package services

import (
	"github.com/go-raptor/raptor/v4"
)

type HelloService struct {
	raptor.Service

	greetings []string
}

func NewHelloService() *HelloService {
	return &HelloService{
		greetings: []string{},
	}
}

func (hs *HelloService) Greeting() string {
	return "Hello, World!"
}

func (hs *HelloService) Greetings() []string {
	return hs.greetings
}

func (hs *HelloService) AddGreeting(greeting string) {
	hs.greetings = append(hs.greetings, greeting)
}
