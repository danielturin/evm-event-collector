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

	"github.com/amirylm/lockfree/core"
	"github.com/amirylm/lockfree/reactor"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
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
	log          *zap.Logger
	data_queue   core.Queue[types.Callback]
}

func New(contractData types.ContractData, reactor reactor.Reactor[types.LogEvent, types.Callback], data_queue core.Queue[types.Callback]) *controller {
	ctrl := &controller{
		reactor:      reactor,
		contractData: contractData,
		mutext:       &sync.Mutex{},
		log:          logger.GetNamedLogger("controller"),
		data_queue:   data_queue,
	}
	return ctrl
}

func (ctrl *controller) Start(contractData types.ContractData) {
	ctrl.log.Info("Iterating over events", zap.Any("events", contractData.Events))
	for _, event := range contractData.Events {
		eventAbi := ""
		current_event := event
		for _, abi := range contractData.ABI {
			if abi.Name == current_event.ABI {
				eventAbi = abi.Data
			}
		}

		contractAbi, _ := abi.JSON(bytes.NewReader([]byte(eventAbi)))
		randomBytes := make([]byte, 32)
		_, err := rand.Read(randomBytes)
		if err != nil {
			ctrl.log.Error("Could not generate random bytes")
		}

		hasher := sha256.New()
		hasher.Write(randomBytes)
		hash := hasher.Sum(nil)

		identifier := hex.EncodeToString(hash)
		rs := &ReactiveServiceImpl{
			SelectLogic: func(e reactor.Event[types.LogEvent]) bool {
				return len(e.Data.ID) > 0 && strings.EqualFold(e.Data.Log.Address.String(), current_event.Addr)
			},
			HandleLogic: func(e reactor.Event[types.LogEvent], callback func(types.Callback, error)) {
				ctrl.log.Info("EventHandler triggered", zap.String("txHash", e.Data.Log.TxHash.Hex()))
				go func(e reactor.Event[types.LogEvent]) {
					cb1 := ctrl.Preprocess(e.Data, contractAbi)
					if cb1 != nil {
						ctrl.log.Info("Preprocess completed", zap.String("txHash", e.Data.Log.TxHash.Hex()))
						callback(*cb1, nil)
					}
				}(e)
			},
		}

		cs := &CallbackServiceImpl{
			SelectLogic: func(c reactor.Event[types.Callback]) bool {
				return len(c.Data.Addr) > 0 && strings.EqualFold(c.Data.Addr.String(), current_event.Addr) &&
					strings.EqualFold(c.Data.EventSigId.String(), current_event.EventSig)
			},
			HandleLogic: func(c reactor.Event[types.Callback]) {
				ctrl.log.Debug("Callback Process triggered", zap.String("txHash", c.Data.TxHash.Hex()))
				go func(c reactor.Event[types.Callback]) {
					ctrl.Process(c.Data)
				}(c)
			},
		}

		ctrl.log.Debug("Adding Callback", zap.String("identifier", identifier+"_callback"))
		ctrl.reactor.AddCallback(identifier+"_callback", cs, 1)

		ctrl.log.Debug("Adding Handler", zap.String("identifier", identifier+"_handler"))
		ctrl.reactor.AddHandler(identifier+"_handler", rs, 1)
	}
}

func (ctrl *controller) Preprocess(e types.LogEvent, contractAbi abi.ABI) *types.Callback {
	ev, err := contractAbi.EventByID(e.Log.Topics[0])
	if err != nil {
		ctrl.log.Error("could not find Event by ID: ", zap.Error(err), zap.String("event_ID", e.ID))
		return nil
	}
	if ev.Name == "Transfer" {
		ctrl.log.Debug("Preprocess: Event fetched by ID and is of type Transfer")

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
			ctrl.log.Warn("Error decoding topic[1] value: ", zap.Error(err))
		}

		// Handle To address
		topic2Bytes, err := hex.DecodeString(e.Log.Topics[2].Hex()[2:])
		if err == nil {
			// Extract last 20 bytes to acquire address
			toAddress := common.BytesToAddress(topic2Bytes[12:])
			cb.To = toAddress
		} else {
			ctrl.log.Warn("Error decoding topic[2] value: ", zap.Error(err))
		}

		amount, err := contractAbi.Unpack("Transfer", e.Log.Data)
		if err == nil {
			bi_amount := amount[0].(*big.Int)
			divisor := big.NewInt(1000000)
			result := new(big.Float).Quo(new(big.Float).SetInt(bi_amount), new(big.Float).SetInt(divisor))
			cb.Amount = *result
		} else {
			ctrl.log.Warn("error unpacking transfer amount: ", zap.Error(err))
		}

		ctrl.data_queue.Enqueue(*cb)
		return cb
	}
	ctrl.log.Debug("Preprocess: event is not of type Transfer, ignoring event: ", zap.String("TxHash", e.Log.TxHash.Hex()))
	return nil
}

func (ctrl *controller) Process(c types.Callback) {
	ctrl.mutext.Lock()
	filePath := "callbacksData.txt"
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0644)

	if err != nil {
		// If the file does not exist, create it
		if os.IsNotExist(err) {
			ctrl.log.Debug("Process: Crearting file")
			file, err = os.Create(filePath)
			if err != nil {
				ctrl.log.Error("Error creating file: ", zap.Error(err))
				return
			}
		} else {
			// If there was an error other than the file not existing, handle it
			ctrl.log.Error("Error opening file: ", zap.Error(err))
			return
		}
	}

	defer file.Close()
	ctrl.log.Debug("Process: Writing to File transaction:  ", zap.String("TxHash", hex.EncodeToString(c.TxHash[:])))
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

	ctrl.log.Debug("Data written to the file successfully.")
	ctrl.mutext.Unlock()
}

func (ctrl *controller) GetDataQueue() core.Queue[types.Callback] {
	return ctrl.data_queue
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
	// Call the logic function for Handle with the  callback as arguments
	cs.HandleLogic(callback)
}
