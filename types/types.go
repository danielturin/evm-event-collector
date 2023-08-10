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
	EventName   string
	EventSig    string
	EventSigId  common.Hash
	From, To    common.Address
	Amount      big.Float
	TxHash      common.Hash
	Addr        common.Address
	BlockHash   common.Hash
	BlockNumber uint64
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
