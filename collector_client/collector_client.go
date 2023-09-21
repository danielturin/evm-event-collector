package collector_client

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/danielturin/evm-event-collector/client"
	"github.com/danielturin/evm-event-collector/logger"
	"github.com/danielturin/evm-event-collector/types"

	"github.com/amirylm/lockfree/reactor"
)

type Collector interface {
	Start() *collector
}

type collector struct {
	CollectorClient client.Client
	Reactor         reactor.Reactor[types.LogEvent, types.Callback]
}

func New() *collector {
	logger.CreateLoggerInstance()
	log := logger.GetNamedLogger("collector_client")

	defer logger.Sync()

	contract_config, err := os.Open("./config.json")
	if err != nil {
		log.Sugar().Errorf("Could not open config.json: ", err)
		panic(err)
	}
	defer contract_config.Close()

	// Read the contents of the file
	data, err := io.ReadAll(contract_config)
	if err != nil {
		log.Sugar().Errorf("Could not read config.json: ", err)
		panic(err)
	}

	// Parse the JSON data into the ContractData struct
	var contractData types.ContractData
	err = json.Unmarshal(data, &contractData)
	if err != nil {
		log.Sugar().Errorf("Could not parse contract data from config.json: ", err)
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reactor := reactor.New(
		reactor.WithCallbacksDemux[types.LogEvent, types.Callback](reactor.NewDemux[reactor.Event[types.Callback]]()),
		reactor.WithEventsDemux[types.LogEvent, types.Callback](reactor.NewDemux[reactor.Event[types.LogEvent]]()))

	go func() {
		_ = reactor.Start(ctx)
	}()

	c := client.New(reactor, contractData)
	cc := &collector{
		CollectorClient: *c,
		Reactor:         reactor,
	}
	return cc
}

func (col *collector) Start(addr string, timeout_duration int64) {
	logger.CreateLoggerInstance()
	log := logger.GetNamedLogger("collector_client")

	defer logger.Sync()

	if len(addr) == 0 {
		log.Error("Please enter a valid websocket SOCKET_ADDRS in .env")
		return
	}

	contract_config, err := os.Open("./config.json")
	if err != nil {
		log.Sugar().Errorf("Could not open config.json: ", err)
		panic(err)
	}
	defer contract_config.Close()

	// Read the contents of the file
	data, err := io.ReadAll(contract_config)
	if err != nil {
		log.Sugar().Errorf("Could not read config.json: ", err)
		panic(err)
	}

	// Parse the JSON data into the ContractData struct
	var contractData types.ContractData
	err = json.Unmarshal(data, &contractData)
	if err != nil {
		log.Sugar().Errorf("Could not parse contract data from config.json: ", err)
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if timeout_duration == 0 {
		timeout_duration = 100000 // default fallback timeout
	}
	timeout := time.Duration(timeout_duration) * time.Millisecond

	col.CollectorClient.Subscriber.Connect(ctx, addr, timeout)
	if err != nil {
		log.Error("failed to establish connection!")
	}
	col.CollectorClient.Controller.Start(contractData)

	log.Info("Invoking Subscriber")
	col.CollectorClient.Subscriber.Subscribe(ctx, col.Reactor, contractData)
}
