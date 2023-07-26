package subscriber

import (
	"context"
	"evm-event-collector/types"
	"fmt"
	"time"

	"github.com/amirylm/lockfree/reactor"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
)

type Filter struct {
	ID       string
	Address  common.Address
	EventSig common.Hash
}

type Subscriber interface {
	Connect(ctx context.Context, addr string, timeout time.Duration) error
	Subscribe(ctx context.Context, contractAbi abi.ABI, reactor reactor.Reactor[types.LogEvent, types.Callback], contractData types.ContractData) error
}

func New(addr string, timeout time.Duration) *subscriber {
	return &subscriber{}
}

type subscriber struct {
	conn *ethclient.Client
}

func (s *subscriber) Connect(pctx context.Context, addr string, timeout time.Duration) error {
	conn, err := s.connect(pctx, addr, timeout)
	if err != nil {
		return err
	}
	s.conn = conn
	return nil
}

func (s *subscriber) connect(pctx context.Context, addr string, timeout time.Duration) (*ethclient.Client, error) {
	fmt.Printf("Establishing connection to %s\n", addr)
	conn, err := ethclient.DialContext(pctx, addr)

	if err != nil {
		fmt.Printf("Connceetion Failure: %+v\n", err)
		return nil, errors.Wrap(err, "connection failure")
	}
	return conn, nil
}

func (s *subscriber) Subscribe(ctx context.Context, reactor reactor.Reactor[types.LogEvent, types.Callback], contractData types.ContractData) error {
	logs := make(chan gethtypes.Log)

	filtersQuery := ethereum.FilterQuery{}
	genFiltersQuery(&filtersQuery, contractData)
	fmt.Println("Subscribing to Filter Logs")
	sub, err := s.conn.SubscribeFilterLogs(ctx, filtersQuery, logs)
	if err != nil {
		fmt.Printf("failed to SubscribeFilterLogs %s", err)
		return nil
	}
	defer sub.Unsubscribe()
	go s.listen(sub, logs, reactor, contractData)
	select {
	case <-ctx.Done():
		fmt.Println("Context terminated")
	case err := <-sub.Err():
		fmt.Printf("Context error: %s\n", err)
	}

	return nil
}

func (s *subscriber) listen(sub ethereum.Subscription, logs chan gethtypes.Log, reactor reactor.Reactor[types.LogEvent, types.Callback], contractData types.ContractData) {
	fmt.Println("Listening for Events")
	for {
		fmt.Println("Waiting for channel input")
		select {
		case log := <-logs:
			fmt.Printf("Received Log for Transaction: %+v\n", log.TxHash)
			go s.processEvent(log, reactor)

		case err := <-sub.Err():
			fmt.Println("Error in listens")
			errors.Wrap(err, "retrieving log from subscription failed")
		}
	}
}

func genFiltersQuery(filters *ethereum.FilterQuery, contractData types.ContractData) {
	for _, eventData := range contractData.Events {
		fmt.Printf("gethFiltersQuery: creating filter %s'\n", eventData)
		filters.Addresses = append(filters.Addresses, common.HexToAddress(eventData.Addr))
	}
	fmt.Printf("gethFiltersQuery: %s'\n", filters)
}

func (s *subscriber) processEvent(log gethtypes.Log, reactor reactor.Reactor[types.LogEvent, types.Callback]) {
	id := log.Address.String()
	e := types.LogEvent{Log: log, ID: id}
	fmt.Printf("processEvent: enqueuing event for TxHash - %+v\n", e.Log.TxHash)
	reactor.Enqueue(e)
}
