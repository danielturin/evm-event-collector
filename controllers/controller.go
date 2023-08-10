package controllers

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	logger "evm-event-collector/logger"
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
	log          *logger.Log
}

// filter is current config + add action (API for notification?)
func New(contractData types.ContractData, reactor reactor.Reactor[types.LogEvent, types.Callback]) *controller {
	ctrl := &controller{
		reactor:      reactor,
		contractData: contractData,
		mutext:       &sync.Mutex{},
		log:          logger.GetNamedLogger("controller"),
	}
	return ctrl
}

func (ctrl *controller) Start(contractData types.ContractData) {
	ctrl.log.Logger.Sugar().Info("Iterating over events: ", contractData.Events)
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
			ctrl.log.Logger.Error("Could not generate random bytes")
		}

		hasher := sha256.New()
		hasher.Write(randomBytes)
		hash := hasher.Sum(nil)

		identifier := hex.EncodeToString(hash)
		ctrl.log.Logger.Sugar().Info("Identifier is: ", identifier)

		rs := &ReactiveServiceImpl{
			SelectLogic: func(e reactor.Event[types.LogEvent]) bool {
				return len(e.Data.ID) > 0 && strings.EqualFold(e.Data.Log.Address.String(), event.Addr)
			},
			HandleLogic: func(e reactor.Event[types.LogEvent], callback func(types.Callback, error)) {
				ctrl.log.Logger.Sugar().Info("EventHandler triggered for: ", e.Data.Log.TxHash)
				cb1 := ctrl.Preprocess(e.Data, contractAbi)
				if cb1 != nil {
					ctrl.log.Logger.Sugar().Info("Preprocess completed for: ", e.Data.Log.TxHash)
					callback(*cb1, nil)
				}
			},
		}

		cs := &CallbackServiceImpl{
			SelectLogic: func(c reactor.Event[types.Callback]) bool {
				return len(c.Data.Addr) > 0 && strings.EqualFold(c.Data.Addr.String(), event.Addr) &&
					strings.EqualFold(c.Data.EventSigId.String(), event.EventSig)
			},
			HandleLogic: func(c reactor.Event[types.Callback]) {
				ctrl.log.Logger.Sugar().Info("Callback Process triggered for: ", c.Data.TxHash.String())
				ctrl.Process(c.Data)
			},
		}

		ctrl.log.Logger.Sugar().Info("Adding Callback for: ", identifier+"_callback")
		ctrl.reactor.AddCallback(identifier+"_callback", cs, 1)

		ctrl.log.Logger.Sugar().Info("Adding Handler for: ", identifier+"_handler")
		ctrl.reactor.AddHandler(identifier+"_handler", rs, 1)
	}
}

func (ctrl *controller) Preprocess(e types.LogEvent, contractAbi abi.ABI) *types.Callback {
	ev, err := contractAbi.EventByID(e.Log.Topics[0])
	if ev.Name == "Transfer" {
		ctrl.log.Logger.Info("Preprocess: Event fetched by ID and is of type Transfer")
		if err != nil {
			ctrl.log.Logger.Sugar().Error("could not find Event by ID: ", err)
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
			ctrl.log.Logger.Sugar().Error("Error decoding topic[1] value: ", err)
		}

		// Handle To address
		topic2Bytes, err := hex.DecodeString(e.Log.Topics[2].Hex()[2:])
		if err == nil {
			// Extract last 20 bytes to acquire address
			toAddress := common.BytesToAddress(topic2Bytes[12:])
			cb.To = toAddress
		} else {
			ctrl.log.Logger.Sugar().Error("Error decoding topic[2] value: ", err)
		}

		amount, err := contractAbi.Unpack("Transfer", e.Log.Data)
		if err == nil {
			bi_amount := amount[0].(*big.Int)
			divisor := big.NewInt(1000000)
			result := new(big.Float).Quo(new(big.Float).SetInt(bi_amount), new(big.Float).SetInt(divisor))
			cb.Amount = *result
		} else {
			ctrl.log.Logger.Sugar().Error("error unpacking transfer amount: ", err)
		}
		return cb
	}
	ctrl.log.Logger.Sugar().Info("Preprocess: event is not of type Transfer, ignoring event: ", e.Log.TxHash)
	return nil
}

// should receive interface of logDataEvent (allows types such as: transfer/burn etc...)
func (ctrl *controller) Process(c types.Callback) {
	ctrl.mutext.Lock()
	filePath := "callbacksData.txt"
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0644)

	if err != nil {
		// If the file does not exist, create it
		if os.IsNotExist(err) {
			ctrl.log.Logger.Info("Process: Crearting file")
			file, err = os.Create(filePath)
			if err != nil {
				ctrl.log.Logger.Sugar().Error("Error creating file: ", err)
				return
			}
		} else {
			// If there was an error other than the file not existing, handle it
			ctrl.log.Logger.Sugar().Error("Error opening file: ", err)
			return
		}
	}

	defer file.Close()
	ctrl.log.Logger.Sugar().Info("Process: Writing to File transaction:  ", hex.EncodeToString(c.TxHash[:]))
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

	ctrl.log.Logger.Info("Data written to the file successfully.")
	ctrl.mutext.Unlock()
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
