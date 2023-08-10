package subscriber

import (
	"context"
	logger "evm-event-collector/logger"
	"evm-event-collector/types"
	"time"

	"github.com/amirylm/lockfree/reactor"
	"github.com/ethereum/go-ethereum"
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
	Subscribe(ctx context.Context, reactor reactor.Reactor[types.LogEvent, types.Callback], contractData types.ContractData) error
}

func New(addr string, timeout time.Duration) *subscriber {
	return &subscriber{
		log: logger.GetNamedLogger("subscriber"),
	}
}

type subscriber struct {
	conn *ethclient.Client
	log  *logger.Log
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
	s.log.Logger.Sugar().Info("Establishing connection to ", addr)
	conn, err := ethclient.DialContext(pctx, addr)

	if err != nil {
		s.log.Logger.Sugar().Error("Connceetion Failure: ", err)
		return nil, errors.Wrap(err, "connection failure")
	}
	return conn, nil
}

func (s *subscriber) Subscribe(ctx context.Context, reactor reactor.Reactor[types.LogEvent, types.Callback], contractData types.ContractData) error {
	logs := make(chan gethtypes.Log)

	filtersQuery := ethereum.FilterQuery{}
	genFiltersQuery(&filtersQuery, contractData)
	s.log.Logger.Info("Subscribing to Filter Logs")
	sub, err := s.conn.SubscribeFilterLogs(ctx, filtersQuery, logs)
	if err != nil {
		s.log.Logger.Sugar().Error("failed to SubscribeFilterLogs ", err)
		return nil
	}
	defer sub.Unsubscribe()
	go s.listen(sub, logs, reactor, contractData)
	select {
	case <-ctx.Done():
		s.log.Logger.Info("Context terminated")
	case err := <-sub.Err():
		s.log.Logger.Sugar().Error("Context error: ", err)
	}

	return nil
}

func (s *subscriber) listen(sub ethereum.Subscription, logs chan gethtypes.Log, reactor reactor.Reactor[types.LogEvent, types.Callback], contractData types.ContractData) {
	s.log.Logger.Info("Listening for Events")
	for {
		s.log.Logger.Info("Waiting for channel input")
		select {
		case log := <-logs:
			s.log.Logger.Sugar().Info("Received Log for Transaction: ", log.TxHash)
			go s.processEvent(log, reactor)

		case err := <-sub.Err():
			s.log.Logger.Sugar().Error("Failed while listening: ", err)
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
	s.log.Logger.Sugar().Info("processEvent: enqueuing event for TxHash - ", e.Log.TxHash)
	reactor.Enqueue(e)
}
