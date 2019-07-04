package jsonrpc2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type FruitService struct{}

func (f *FruitService) Apple() string {
	return "Apple"
}

func (f *FruitService) Banana() error {
	return nil
}

func (f *FruitService) Cherry() (string, error) {
	return "Cherry", nil
}

func (f *FruitService) Durian() error {
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
	err := f.PongService.Call(context.Background(), &pong, "pong")
	if err != nil {
		return fmt.Sprintf("err: %s", err)
	}
	return "ping" + pong
}

type Ponger struct{}

func (b *Ponger) Pong() string {
	return "pong"
}

type Fib struct{}

func (f *Fib) Fibonacci(ctx context.Context, a int, b int, steps int) (int, error) {
	service, err := CtxService(ctx)
	if err != nil {
		return 0, err
	}
	a, b = b, a+b
	if steps <= 0 {
		return b, nil
	}
	if err := service.Call(ctx, &b, "fibonacci", a, b, steps-1); err != nil {
		return 0, err
	}
	return b, nil
}

func marshalEqual(a, b interface{}) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Compare(aa, bb) == 0
}

type BatchByID []Message

func (b BatchByID) Len() int {
	return len(b)
}

func (b BatchByID) Less(i, j int) bool {
	return bytes.Compare(b[i].ID, b[j].ID) < 0
}

func (b BatchByID) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}
