package client

import (
	"github.com/danielturin/evm-event-collector/controllers"
	"github.com/danielturin/evm-event-collector/subscriber"
	"github.com/danielturin/evm-event-collector/types"

	"github.com/amirylm/lockfree/core"
	"github.com/amirylm/lockfree/queue"
	"github.com/amirylm/lockfree/reactor"
)

// type Client interface {
// 	New(addr string, timeout time.Duration,
// 		reactor reactor.Reactor[types.LogEvent, types.Callback],
// 		contractData types.ContractData) *client
// }

type Client struct {
	Subscriber subscriber.Subscriber
	Controller controllers.Controller
	Data_Queue core.Queue[types.Callback]
}

func New(reactor reactor.Reactor[types.LogEvent, types.Callback],
	contractData types.ContractData) *Client {
	data_queue := queue.New(queue.WithCapacity[types.Callback](32768))
	return &Client{
		Subscriber: subscriber.New(),
		Controller: controllers.New(contractData, reactor, data_queue),
		Data_Queue: data_queue,
	}
}

func (cl *Client) GetDataQueue() core.Queue[types.Callback] {
	return cl.Data_Queue
}
