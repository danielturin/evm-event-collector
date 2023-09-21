package subscriber

import (
	"context"
	"time"

	"github.com/danielturin/evm-event-collector/types"

	logger "github.com/danielturin/evm-event-collector/logger"

	"github.com/amirylm/lockfree/reactor"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Filter struct {
	ID       string
	Address  common.Address
	EventSig common.Hash
}

type Subscriber interface {
	Connect(ctx context.Context, addr string, timeout time.Duration) error
	Subscribe(ctx context.Context, reactor reactor.Reactor[types.LogEvent, types.Callback], contractData types.ContractData) error
}

func New() *subscriber {
	return &subscriber{
		log: logger.GetNamedLogger("subscriber"),
	}
}

type subscriber struct {
	conn *ethclient.Client
	log  *zap.Logger
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
	s.log.Info("Establishing connection to ", zap.String("Address", addr))
	conn, err := ethclient.DialContext(pctx, addr)

	if err != nil {
		s.log.Error("Connection Failure: ", zap.Error(err))
		return nil, errors.Wrap(err, "connection failure")
	}
	return conn, nil
}

func (s *subscriber) Subscribe(ctx context.Context, reactor reactor.Reactor[types.LogEvent, types.Callback], contractData types.ContractData) error {
	logs := make(chan gethtypes.Log)

	filtersQuery := ethereum.FilterQuery{}
	genFiltersQuery(&filtersQuery, contractData)
	s.log.Debug("Subscribing to Filter Logs")
	sub, err := s.conn.SubscribeFilterLogs(ctx, filtersQuery, logs)
	if err != nil {
		s.log.Error("failed to SubscribeFilterLogs ", zap.Error(err))
		return nil
	}
	defer sub.Unsubscribe()
	go s.listen(sub, logs, reactor)
	select {
	case <-ctx.Done():
		s.log.Debug("Context terminated")
	case err := <-sub.Err():
		s.log.Error("Context error: ", zap.Error(err))
	}

	return nil
}

func (s *subscriber) listen(sub ethereum.Subscription, logs chan gethtypes.Log, reactor reactor.Reactor[types.LogEvent, types.Callback]) {
	s.log.Debug("Listening for Events")
	for {
		s.log.Debug("Waiting for channel input")
		select {
		case log := <-logs:
			s.log.Debug("Received Log for Transaction: ", zap.String("TxHash", log.TxHash.Hex()))
			go s.processEvent(log, reactor)

		case err := <-sub.Err():
			s.log.Error("Failed while listening: ", zap.Error(err))
			errors.Wrap(err, "retrieving log from subscription failed")
		}
	}
}

func genFiltersQuery(filters *ethereum.FilterQuery, contractData types.ContractData) {
	for _, eventData := range contractData.Events {
		filters.Addresses = append(filters.Addresses, common.HexToAddress(eventData.Addr))
	}
}

func (s *subscriber) processEvent(log gethtypes.Log, reactor reactor.Reactor[types.LogEvent, types.Callback]) {
	id := log.Address.String()
	e := types.LogEvent{Log: log, ID: id}
	s.log.Debug("processEvent: enqueuing event for TxHash - ", zap.String("TxHash", e.Log.TxHash.Hex()))
	reactor.Enqueue(e)
}
