package evm2

import (
	"context"
	"fmt"

	"github.com/Conflux-Chain/go-conflux-util/blockchain/sync/evm"
	"github.com/mcuadros/go-defaults"
	providers "github.com/openweb3/go-rpc-provider/provider_wrapper"
	"github.com/openweb3/web3go"
	"github.com/openweb3/web3go/types"
	"github.com/pkg/errors"
)

// EvmAdapter implements the poll.Adapter[T] interface to poll data from evm RPC.
type EvmAdapter struct {
	option evm.AdapterOption

	client *web3go.Client
}

func NewEvmAdapter(url string, option evm.AdapterOption) (*EvmAdapter, error) {
	defaults.SetDefaults(&option)

	clientOption := web3go.ClientOption{
		Option: providers.Option{
			RequestTimeout: option.RequestTimeout,
		},
	}

	client, err := web3go.NewClientWithOption(url, clientOption)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to create client")
	}

	return &EvmAdapter{option, client}, nil
}

func NewEvmAdapterWithConfig(config evm.AdapterConfig) (*EvmAdapter, error) {
	if len(config.URL) == 0 {
		return nil, errors.New("URL not specified")
	}

	return NewEvmAdapter(config.URL, config.AdapterOption)
}

// Close closes the underlying RPC client.
func (adapter *EvmAdapter) Close() {
	adapter.client.Close()
}

// GetFinalizedBlockNumber implements the poll.Adapter[T] interface.
func (adapter *EvmAdapter) GetFinalizedBlockNumber(ctx context.Context) (uint64, error) {
	block, err := adapter.client.WithContext(ctx).Eth.BlockByNumber(types.FinalizedBlockNumber, false)
	if err != nil {
		return 0, err
	}

	return block.Number.Uint64(), nil
}

// GetLatestBlockNumber implements the poll.Adapter[T] interface.
func (adapter *EvmAdapter) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	block, err := adapter.client.WithContext(ctx).Eth.BlockByNumber(types.BlockNumber(adapter.option.LatestBlockNumberTag), false)
	if err != nil {
		return 0, err
	}

	bn := block.Number.Uint64()
	if bn < adapter.option.LatestBlockNumberOffset {
		return 0, nil
	}

	return bn - adapter.option.LatestBlockNumberOffset, nil
}

// GetBlockData implements the poll.Adapter[T] interface.
func (adapter *EvmAdapter) GetBlockData(ctx context.Context, blockNumber uint64) (evm.BlockData, error) {
	var data evm.BlockData

	bn := types.BlockNumber(blockNumber)

	block, err := adapter.client.Eth.BlockByNumber(bn, true)
	if err != nil {
		return data, errors.WithMessage(err, "Failed to get block by number")
	}
	data.Block = block

	if block == nil {
		return data, errors.Errorf("Block not found by number %v", blockNumber)
	}

	if !adapter.option.IgnoreReceipts {
		if err := queryReceipts(&data, adapter.client, bn); err != nil {
			return data, errors.WithMessage(err, fmt.Sprintf("Failed to query receipts at %d", blockNumber))
		}
	}

	if !adapter.option.IgnoreTraces {
		if err := queryTraces(&data, adapter.client, bn); err != nil {
			return data, errors.WithMessage(err, fmt.Sprintf("Failed to query traces at %d", blockNumber))
		}
	}

	return data, nil
}

func queryTraces(data *evm.BlockData, client *web3go.Client, blockNumber types.BlockNumber) error {
	txs := data.Block.Transactions.Transactions()
	if len(txs) == 0 {
		data.Traces = []types.LocalizedTrace{}
		return nil
	}

	bnoh := types.BlockNumberOrHashWithHash(data.Block.Hash, true)
	traces, err := client.Trace.Blocks(bnoh)
	if err != nil {
		return errors.WithMessage(err, "Failed to get block traces by block number")
	}

	if traces == nil {
		return errors.Errorf("Traces not found by block %v", blockNumber.Int64())
	}

	// Try to detect temp chain reorg if there is any trace.
	// Otherwise, temp chain reorg may lead to data inconsistency issue.
	for i, v := range traces {
		if v.BlockHash != data.Block.Hash {
			return errors.Errorf("Trace block hash mismatch, index = %v", i)
		}
	}

	data.Traces = traces

	return nil
}

func queryReceipts(data *evm.BlockData, client *web3go.Client, blockNumber types.BlockNumber) error {
	txs := data.Block.Transactions.Transactions()
	if len(txs) == 0 {
		data.Receipts = []*types.Receipt{}
		return nil
	}

	bnoh := types.BlockNumberOrHashWithNumber(blockNumber)
	receipts, err := client.Eth.BlockReceipts(&bnoh)
	if err != nil {
		return errors.WithMessage(err, "Failed to get block receipts by block number")
	}

	if receipts == nil {
		return errors.Errorf("Receipts not found by block %v", blockNumber.Int64())
	}

	// detect temp chain reorg
	if len(receipts) != len(txs) {
		return errors.Errorf("Receipts length and txs length mismatch, receipts = %v, txs = %v", len(receipts), len(txs))
	}

	for i, v := range receipts {
		if v.BlockHash != data.Block.Hash {
			return errors.Errorf("Receipt block hash mismatch, index = %v", i)
		}

		if v.TransactionHash != txs[i].Hash {
			return errors.Errorf("Receipt tx hash mismatch, index = %v", i)
		}
	}

	data.Receipts = receipts

	return nil
}

// GetBlockHash implements the poll.Adapter[T] interface.
func (adapter *EvmAdapter) GetBlockHash(data evm.BlockData) string {
	return data.Block.Hash.Hex()
}

// GetParentBlockHash implements the poll.Adapter[T] interface.
func (adapter *EvmAdapter) GetParentBlockHash(data evm.BlockData) string {
	return data.Block.ParentHash.Hex()
}
