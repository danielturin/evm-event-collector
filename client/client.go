package client

import (
	"evm-event-collector/controllers"
	"evm-event-collector/subscriber"
	"evm-event-collector/types"
	"time"

	"github.com/amirylm/lockfree/reactor"
)

type Client interface {
	Start()
	New(addr string, timeout time.Duration,
		reactor reactor.Reactor[types.LogEvent, types.Callback],
		contractData types.ContractData) *client
}

type client struct {
	Subscriber subscriber.Subscriber
	Controller controllers.Controller
}

func New(addr string, timeout time.Duration,
	reactor reactor.Reactor[types.LogEvent, types.Callback],
	contractData types.ContractData) *client {
	return &client{
		Subscriber: subscriber.New(addr, timeout),
		Controller: controllers.New(contractData, reactor),
	}
}
