package collector_client

import (
	"context"
	"encoding/json"
	client "evm-event-collector/client"
	"evm-event-collector/logger"
	"evm-event-collector/types"
	"io"
	"os"
	"time"

	"github.com/amirylm/lockfree/reactor"
)

type collector_client struct {
	Client client.Client
}

func (cl *collector_client) Start(socket_addr interface{}, timeout_duration interface{}) {
	logger.CreateLoggerInstance()
	log := logger.GetNamedLogger("collector_client")

	defer logger.Sync()
	// viper.SetConfigFile(".env")
	// viper.ReadInConfig()
	addr := socket_addr
	timeout_env := timeout_duration

	if len(addr.(string)) == 0 {
		log.Error("Please enter a valid websocket SOCKET_ADDRS in .env")
		return
	}

	contract_config, err := os.Open("config.json")
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

	timeoutInt64, ok := timeout_env.(int64)
	if !ok {
		timeoutInt64 = 100000 // default fallback timeout
	}
	timeout := time.Duration(timeoutInt64) * time.Millisecond

	c := client.New(addr.(string), timeout, reactor, contractData)
	c.Subscriber.Connect(ctx, addr.(string), timeout)
	if err != nil {
		log.Error("failed to establish connection!")
	}
	c.Controller.Start(contractData)

	log.Info("Invoking Subscriber")
	c.Subscriber.Subscribe(ctx, reactor, contractData)
}
