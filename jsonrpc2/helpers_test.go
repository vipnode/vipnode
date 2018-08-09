package jsonrpc2

import (
	"errors"
	"fmt"
)

type FruitService struct{}

func (_ *FruitService) Apple() string {
	return "Apple"
}

func (_ *FruitService) Banana() error {
	return nil
}

func (_ *FruitService) Cherry() (string, error) {
	return "Cherry", nil
}

func (_ *FruitService) Durian() error {
	return errors.New("durian failure")
}

type Pinger struct {
	PongService Service
}

func (f *Pinger) Ping() string {
	return "ping"
}

func (f *Pinger) PingPong() string {
	var pong string
	err := f.PongService.Call(&pong, "pong")
	if err != nil {
		return fmt.Sprintf("err: %s", err)
	}
	return "ping" + pong
}

type Ponger struct{}

func (b *Ponger) Pong() string {
	return "pong"
}
