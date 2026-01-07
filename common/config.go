package common

import (
	"github.com/Conflux-Chain/go-conflux-util/blockchain/sync"
	"github.com/Conflux-Chain/go-conflux-util/blockchain/sync/core"
	"github.com/Conflux-Chain/go-conflux-util/blockchain/sync/evm"
)

//blockchain/sync/sync_db.go

type Config[T evm.BlockData | core.EpochData] struct {
	sync.CatchupParamsDB[T]
	sync.ParamsDB[T]
	evm.AdapterConfig
	FirstBlock uint64
	Batch      bool
}

type EvmConfig struct {
	Config[evm.BlockData]
}

type CfxConfig struct {
	Config[core.EpochData]
}
