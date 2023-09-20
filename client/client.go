package client

import (
	"evm-event-collector/controllers"
	"evm-event-collector/subscriber"
	"evm-event-collector/types"
	"time"

	"github.com/amirylm/lockfree/core"
	"github.com/amirylm/lockfree/queue"
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
	Data_Queue core.Queue[types.Callback]
}

func New(addr string, timeout time.Duration,
	reactor reactor.Reactor[types.LogEvent, types.Callback],
	contractData types.ContractData) *client {
	data_queue := queue.New(queue.WithCapacity[types.Callback](32768))
	return &client{
		Subscriber: subscriber.New(addr, timeout),
		Controller: controllers.New(contractData, reactor, data_queue),
		Data_Queue: data_queue,
	}
}

func (cl *client) GetDataQueue() core.Queue[types.Callback] {
	return cl.Data_Queue
}
