package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/danielturin/evm-event-collector/types"

	"github.com/danielturin/evm-event-collector/logger"

	client "github.com/danielturin/evm-event-collector/client"

	"github.com/amirylm/lockfree/reactor"
	"github.com/spf13/viper"
)

func main() {
	logger.CreateLoggerInstance()
	log := logger.GetNamedLogger("main")

	defer logger.Sync()
	viper.SetConfigFile(".env")
	viper.ReadInConfig()
	addr := viper.Get("SOCKET_ADDRS")
	timeout_env := viper.Get("TIMEOUT_DURATION")

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

	c := client.New(reactor, contractData)
	c.Subscriber.Connect(ctx, addr.(string), timeout)
	if err != nil {
		log.Error("failed to establish connection!")
	}
	inbound_callbacks := make(chan types.Callback)
	c.Controller.Start(contractData, inbound_callbacks)

	log.Info("Invoking Subscriber")
	c.Subscriber.Subscribe(ctx, reactor, contractData)
}
