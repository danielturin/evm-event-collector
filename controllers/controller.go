package controllers

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
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
	Start(contractDate types.ContractData)
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
	return ctrl
}

func (ctrl *controller) Start(contractData types.ContractData) {
	fmt.Printf("Iterating over events: %+v\n", contractData.Events)
	for _, event := range contractData.Events {
		eventAbi := ""
		for _, abi := range contractData.ABI {
			if abi.Name == event.ABI {
				eventAbi = abi.Data
			}
		}
		contractAbi, _ := abi.JSON(bytes.NewReader([]byte(eventAbi)))

		randomBytes := make([]byte, 32)
		_, err := rand.Read(randomBytes)
		if err != nil {
			fmt.Println("Could not generate random bytes")
		}

		hasher := sha256.New()
		hasher.Write(randomBytes)
		hash := hasher.Sum(nil)

		identifier := hex.EncodeToString(hash)
		fmt.Printf("Identifier is: %s\n", identifier)

		rs := &ReactiveServiceImpl{
			SelectLogic: func(e reactor.Event[types.LogEvent]) bool {
				v := len(e.Data.ID) > 0 && strings.EqualFold(e.Data.Log.Address.String(), event.Addr)
				fmt.Printf("INSIDE Handler Selector Bool value is: %+v\n", v)
				return v
			},
			HandleLogic: func(e reactor.Event[types.LogEvent], callback func(types.Callback, error)) {
				fmt.Printf("EventHandler triggered: %s\n", e.Data.Log.Address)
				cb1 := ctrl.Preprocess(e.Data, contractAbi)
				if cb1 != nil {
					cb := *cb1
					fmt.Printf("Preprocess completed\n")
					fmt.Printf("Callback TxHash: %s\n", cb.TxHash.String())
					fmt.Printf("Callback EventSigId: %s\n", cb.EventSigId)
					callback(*cb1, nil)
				}
			},
		}

		cs := &CallbackServiceImpl{
			SelectLogic: func(c reactor.Event[types.Callback]) bool {
				fmt.Printf("INSIDE: Callback Selector before running selector\n")
				v := len(c.Data.Addr) > 0 && strings.EqualFold(c.Data.Addr.String(), event.Addr) &&
					strings.EqualFold(c.Data.EventSigId.String(), event.EventSig)
				fmt.Printf("INSIDE: Callback Selector Bool values is: %v\n", v)
				return v
			},
			HandleLogic: func(c reactor.Event[types.Callback]) {
				fmt.Printf("Callback Process triggered for %s\n", c.Data.TxHash.String())
				ctrl.Process(c.Data)
			},
		}

		fmt.Printf("Adding Callback for %s\n", identifier)
		ctrl.reactor.AddCallback(identifier+"_callback", cs, 1)

		fmt.Printf("Adding Handler for %s\n", identifier)
		ctrl.reactor.AddHandler(identifier+"_handler", rs, 1)
	}
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
		if err == nil {
			bi_amount := amount[0].(*big.Int)
			divisor := big.NewInt(1000000)
			result := new(big.Float).Quo(new(big.Float).SetInt(bi_amount), new(big.Float).SetInt(divisor))
			cb.Amount = *result
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
	fmt.Fprintf(file, "Amount: %s\n", c.Amount.Text('f', 10))
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

type ReactiveServiceImpl struct {
	SelectLogic func(event reactor.Event[types.LogEvent]) bool
	HandleLogic func(event reactor.Event[types.LogEvent], callback func(types.Callback, error))
}

func (rs *ReactiveServiceImpl) Select(event reactor.Event[types.LogEvent]) bool {
	// Call the logic function for Select with the event as an argument
	return rs.SelectLogic(event)
}

func (rs *ReactiveServiceImpl) Handle(event reactor.Event[types.LogEvent], callback func(types.Callback, error)) {
	// Call the logic function for Handle with the event and callback as arguments
	rs.HandleLogic(event, callback)
}

type CallbackServiceImpl struct {
	SelectLogic func(event reactor.Event[types.Callback]) bool
	HandleLogic func(callback reactor.Event[types.Callback])
}

func (cs *CallbackServiceImpl) Select(event reactor.Event[types.Callback]) bool {
	// Call the logic function for Select with the event as an argument
	return cs.SelectLogic(event)
}

func (cs *CallbackServiceImpl) Handle(callback reactor.Event[types.Callback]) {
	// Call the logic function for Handle with the event and callback as arguments
	cs.HandleLogic(callback)
}
