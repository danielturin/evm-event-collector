package main

import (
	"context"
	"encoding/json"
	controller "evm-event-collector/controllers"
	subscriber "evm-event-collector/subscriber"
	"evm-event-collector/types"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/amirylm/lockfree/reactor"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigFile(".env")
	viper.ReadInConfig()
	addr := viper.Get("SOCKET_ADDRS")
	timeout_env := viper.Get("TIMEOUT_DURATION")

	if len(addr.(string)) == 0 {
		fmt.Println("Please enter a valid websocket SOCKET_ADDRS in .env")
		return
	}

	contract_config, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	defer contract_config.Close()

	// Read the contents of the file
	data, err := io.ReadAll(contract_config)
	if err != nil {
		panic(err)
	}

	// Parse the JSON data into the ContractData struct
	var contractData types.ContractData
	err = json.Unmarshal(data, &contractData)
	if err != nil {
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

	sub := subscriber.New(addr.(string), timeout)
	controller.New(contractData, reactor)

	err = sub.Connect(ctx, addr.(string), timeout)
	if err != nil {
		fmt.Println("failed to connect!")
	}

	fmt.Println("Calling Subscribe")
	sub.Subscribe(ctx, reactor, contractData)
}
