package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/shurcooL/graphql"
)

type Transaction struct {
	BlockNumber string
	TxHash      string
	LogIndex    string
}

type Pools struct {
	Tokens Tokens
}

type FilterCfg struct {
	ContractData ContractData
	ABI          ABIEntry
}

type LogEvent struct {
	Log types.Log
	ID  string
}

type Callback struct {
	EventName   string         `json:"eventName"`
	EventSig    string         `json:"eventSig"`
	EventSigId  common.Hash    `json:"eventSigId"`
	From        common.Address `json:"from"`
	To          common.Address `json:"to"`
	Amount      big.Float      `json:"amount"`
	TxHash      common.Hash    `json:"txHash"`
	Addr        common.Address `json:"address"`
	BlockHash   common.Hash    `json:"blockHash"`
	BlockNumber uint64         `json:"blockNumber"`
}

type ABIEntry struct {
	Name        string `json:"name"`
	Data        string `json:"data"`
	ABIFilePath string `json:"abiFilePath"`
}

type EventEntry struct {
	Addr     string `json:"addr"`
	EventSig string `json:"eventSig"`
	ABI      string `json:"abi"`
}

type ContractData struct {
	ABI    []ABIEntry   `json:"ABI"`
	Events []EventEntry `json:"events"`
}

type Tokens []struct {
	Symbol         graphql.String
	WhitelistPools []struct {
		Id      graphql.String
		FeeTier graphql.String
		Token0  struct {
			Id     graphql.String
			Symbol graphql.String
		}
		Token1 struct {
			Id     graphql.String
			Symbol graphql.String
		}
	}
}
