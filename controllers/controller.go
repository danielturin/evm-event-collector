package controllers

import (
	"bytes"
	"encoding/hex"
	"evm-event-collector/types"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"

	"github.com/amirylm/lockfree/reactor"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type Controller interface {
	Preprocess(e types.LogEvent, contractAbi abi.ABI) *types.Callback
	Process(c types.Callback)
}

type controller struct {
	reactor      reactor.Reactor[types.LogEvent, types.Callback]
	contractData types.ContractData
	mutext       *sync.Mutex
}

// filter is current config + add action (API for notification?)
func New(contractData types.ContractData, reactor reactor.Reactor[types.LogEvent, types.Callback]) *controller {
	ctrl := &controller{
		reactor:      reactor,
		contractData: contractData,
		mutext:       &sync.Mutex{},
	}
	fmt.Printf("Iterating over events: %+v", contractData.Events)
	for _, event := range contractData.Events {
		eventAbi := ""
		for _, abi := range contractData.ABI {
			if abi.Name == event.ABI {
				eventAbi = abi.Data
			}
		}
		contractAbi, _ := abi.JSON(bytes.NewReader([]byte(eventAbi)))
		identifier := fmt.Sprintf("%s:%s", event.Addr, event.EventSig)
		fmt.Printf("Identifier is: %s\n", identifier)

		fmt.Println("Adding Handler...")
		ctrl.reactor.AddHandler(identifier, func(e types.LogEvent) bool {
			v := len(e.ID) > 0 && strings.EqualFold(e.Log.Address.String(), event.Addr)
			fmt.Printf("INSIDE Handler Selector Bool value is: %+v\n", v)
			return v
		}, 1, func(e types.LogEvent, callback func(types.Callback, error)) {
			go func(e types.LogEvent) {
				fmt.Printf("EventHandler triggered: %s\n", e.Log.Address)
				cb1 := ctrl.Preprocess(e, contractAbi)
				if cb1 != nil {
					cb := *cb1
					fmt.Printf("Preprocess completed\n")
					fmt.Printf("Callback TxHash: %s\n", cb.TxHash.String())
					fmt.Printf("Callback EventSigId: %s\n", cb.EventSigId)
					callback(*cb1, nil)
				}
			}(e)
		})

		fmt.Printf("Adding Callback for %s\n", identifier)
		ctrl.reactor.AddCallback(identifier, func(c types.Callback) bool {
			v := len(c.Addr) > 0 && strings.EqualFold(c.Addr.String(), event.Addr) &&
				strings.EqualFold(c.EventSigId.String(), event.EventSig)
			fmt.Printf("INSIDE: Callback Selector Bool values is: %v\n", v)
			return v
		}, 1, func(c types.Callback) {
			go func(c types.Callback) {
				fmt.Printf("Callback Process triggered for %s\n", c.TxHash.String())
				ctrl.Process(c)
			}(c)

		})
	}
	return ctrl
}

func (ctrl *controller) Preprocess(e types.LogEvent, contractAbi abi.ABI) *types.Callback {
	fmt.Println("Preprocess: handling event")
	ev, err := contractAbi.EventByID(e.Log.Topics[0])
	if ev.Name == "Transfer" {
		fmt.Println("Preprocess: Event fetched by ID and is of type Transfer")
		if err != nil {
			fmt.Println("could not find Event by ID")
			return nil
		}
		cb := &types.Callback{
			EventName:   ev.Name,
			EventSigId:  ev.ID,
			EventSig:    ev.Sig,
			TxHash:      e.Log.TxHash,
			Addr:        e.Log.Address,
			BlockHash:   e.Log.BlockHash,
			BlockNumber: e.Log.BlockNumber,
		}

		// Handle From address
		topic1Bytes, err := hex.DecodeString(e.Log.Topics[1].Hex()[2:])
		if err == nil {
			// Extract last 20 bytes to acquire address
			fromAddress := common.BytesToAddress(topic1Bytes[12:])
			cb.From = fromAddress
		} else {
			fmt.Println("Error decoding topic[1] value:", err)
		}

		// Handle To address
		topic2Bytes, err := hex.DecodeString(e.Log.Topics[2].Hex()[2:])
		if err == nil {
			// Extract last 20 bytes to acquire address
			toAddress := common.BytesToAddress(topic2Bytes[12:])
			cb.To = toAddress
		} else {
			fmt.Println("Error decoding topic[2] value:", err)
		}

		amount, err := contractAbi.Unpack("Transfer", e.Log.Data)
		fmt.Printf("AMOUNT IS: %d\n", amount[0])
		if err == nil {
			cb.Amount = amount[0].(*big.Int)
		} else {
			fmt.Printf("error unpacking transfer amount: %s\n", err)
		}
		fmt.Println("Preprocess: Callback creation complete")
		return cb
	}
	fmt.Println("Preprocess: event is not of type Transfer, returning nil")
	return nil
}

// should receive interface of logDataEvent (allows types such as: transfer/burn etc...)
func (ctrl *controller) Process(c types.Callback) {
	fmt.Println("Locking mutex")
	ctrl.mutext.Lock()
	fmt.Println("Process: Starting to Process callback data")
	filePath := "callbacksData.txt"
	fmt.Println("Process: Attempting to open file")
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0644)

	if err != nil {
		// If the file does not exist, create it
		if os.IsNotExist(err) {
			fmt.Println("Process: Crearting file")
			file, err = os.Create(filePath)
			if err != nil {
				fmt.Println("Error creating file:", err)
				return
			}
		} else {
			// If there was an error other than the file not existing, handle it
			fmt.Println("Error opening file:", err)
			return
		}
	}

	defer file.Close()
	fmt.Println("Process: Writing to File")
	// Write the new line to the file
	fmt.Fprintf(file, "EventName: %s\n", c.EventName)
	fmt.Fprintf(file, "EventSig: %s\n", c.EventSig)
	fmt.Fprintf(file, "EventSigId: %s\n", hex.EncodeToString(c.EventSigId[:]))
	fmt.Fprintf(file, "From: %s\n", hex.EncodeToString(c.From[:]))
	fmt.Fprintf(file, "To: %s\n", hex.EncodeToString(c.To[:]))
	fmt.Fprintf(file, "Amount: %d\n", c.Amount)
	fmt.Fprintf(file, "TxHash: %s\n", hex.EncodeToString(c.TxHash[:]))
	fmt.Fprintf(file, "Addr: %s\n", hex.EncodeToString(c.Addr[:]))
	fmt.Fprintf(file, "BlockHash: %s\n", hex.EncodeToString(c.BlockHash[:]))
	fmt.Fprintf(file, "BlockNumber: %d\n", c.BlockNumber)
	fmt.Fprintf(file, "\n")

	fmt.Println("Data written to the file successfully.")

	fmt.Println("Line appended to existing file.")
	ctrl.mutext.Unlock()
	fmt.Println("Unlocked mutex")
}
