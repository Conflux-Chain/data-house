package common

import "github.com/Conflux-Chain/go-conflux-util/blockchain/sync/evm"

type EvmConfig struct {
	FirstBlock uint64
	evm.Config
	evm.CatchUpConfig
}
