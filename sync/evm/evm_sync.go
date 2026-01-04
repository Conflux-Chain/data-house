package evm

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/Conflux-Chain/data-house/common"
	"github.com/Conflux-Chain/data-house/model"
	evmUtil "github.com/Conflux-Chain/go-conflux-util/blockchain/sync/evm"
	dbUtil "github.com/Conflux-Chain/go-conflux-util/blockchain/sync/process/db"
	"github.com/openweb3/web3go/types"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type TraceProcessor struct {
	db *gorm.DB
}

type TraceOperation struct {
	dbBlock   *model.Block
	minerAddr *model.Address
	traceArr  []*model.Trace
	txArr     []*model.Tx
	Err       error
}

func newErrOperation(err error) *TraceOperation {
	return &TraceOperation{
		Err: err,
	}
}

func (o *TraceOperation) Exec(tx *gorm.DB) error {
	if o.Err != nil {
		return o.Err
	}
	// set miner addr of block
	addrPre, err := model.MakeAddrId(tx, o.minerAddr.Address, o.dbBlock.CreatedAt)
	if err != nil {
		return err
	}
	o.dbBlock.MinerID = addrPre.ID

	//save block
	if err := tx.Create(&o.dbBlock).Error; err != nil {
		return errors.Wrap(err, "create block failed")
	}

	// update block id for tx record
	for _, txPO := range o.txArr {
		txPO.BlockId = o.dbBlock.ID
	}

	if len(o.txArr) > 0 {
		if err := tx.Create(&o.txArr).Error; err != nil {
			return errors.Wrap(err, "create tx")
		}
	}

	// update tx id for trace record
	for _, trace := range o.traceArr {
		trace.TxId = o.txArr[trace.TransactionPosition].ID
	}

	if len(o.txArr) > 0 {
		if err := tx.Create(&o.traceArr).Error; err != nil {
			return errors.Wrap(err, "create trace")
		}
	}

	return nil
}

func (t *TraceProcessor) convertTrace(dbTrace *model.Trace, from string, to string,
	value *big.Int, blockTime time.Time) error {
	fromAddr, err := model.MakeAddrId(t.db, from, blockTime)
	if err != nil {
		return err
	}
	toAddr, err := model.MakeAddrId(t.db, to, blockTime)
	if err != nil {
		return err
	}

	dbTrace.FromId = fromAddr.ID
	dbTrace.ToId = toAddr.ID
	dbTrace.Value = decimal.NewFromBigInt(value, -18)

	return nil
}

func (t *TraceProcessor) Process(data evmUtil.BlockData) dbUtil.Operation {
	blockTime := time.Unix(int64(data.Block.Timestamp), 0)

	txArr, err := buildTxFromReceipt(t.db, data.Receipts, blockTime)
	if err != nil {
		return newErrOperation(errors.Wrap(err, "failed to build tx from receipts"))
	}

	dbBlock := model.Block{
		Model: model.Model{
			ID:        data.Block.Number.Uint64(),
			CreatedAt: blockTime,
		},
		Hash:    data.Block.Hash.Hex(),
		MinerID: 0,
		TxCount: len(data.Receipts),
	}

	dbAddr := model.Address{
		Address:   data.Block.Miner.Hex(),
		BlockTime: dbBlock.CreatedAt,
	}
	// parse trace
	var traceArr []*model.Trace
	var contractLifecycleArr []*model.ContractLifecycle
	for index, trace := range data.Traces {
		dbTrace := model.Trace{
			TraceIndex:          index,
			TraceType:           trace.Type,
			Valid:               *trace.Valid,
			TransactionPosition: *trace.TransactionPosition,
			//Type: trace.Action
		}
		switch trace.Type {
		case types.TRACE_CALL:
			call, _ := trace.Action.(types.Call)
			if err := t.convertTrace(&dbTrace, call.From.Hex(), call.To.Hex(), call.Value, blockTime); err != nil {
				return newErrOperation(err)
			}
			dbTrace.ExtraField = "callType"
			dbTrace.ExtraValue = string(call.CallType)
			dbTrace.Method = common.GetMethodID(call.Input)
		case types.TRACE_CREATE:
			create, _ := trace.Action.(types.Create)
			result, _ := trace.Result.(types.CreateResult)
			if err := t.convertTrace(&dbTrace, create.From.Hex(), result.Address.Hex(), create.Value, blockTime); err != nil {
				return newErrOperation(err)
			}
			dbTrace.ExtraField = "createType"
			if create.CreateType == nil {
				dbTrace.ExtraValue = ""
			} else {
				dbTrace.ExtraValue = string(*create.CreateType)
			}
			contractLifecycleArr = append(contractLifecycleArr, &model.ContractLifecycle{
				Model: model.Model{
					CreatedAt: dbBlock.CreatedAt,
				},
				TxSenderId: txArr[*trace.TransactionPosition].FromId,
				ContractId: dbTrace.ToId,
				Value:      decimal.NewFromBigInt(create.Value, -18),
				Event:      model.ContractLifecycleCreate,
			})
		case types.TRACE_SUICIDE:
			suicide, _ := trace.Action.(types.Suicide)
			if err := t.convertTrace(&dbTrace, "", suicide.Address.Hex(), suicide.Balance, blockTime); err != nil {
				return newErrOperation(err)
			}
			dbTrace.ExtraField = "refundAddress"
			dbTrace.ExtraValue = suicide.RefundAddress.Hex()
			contractLifecycleArr = append(contractLifecycleArr, &model.ContractLifecycle{
				Model: model.Model{
					CreatedAt: dbBlock.CreatedAt,
				},
				TxSenderId: txArr[*trace.TransactionPosition].FromId,
				ContractId: dbTrace.ToId,
				Value:      decimal.NewFromBigInt(suicide.Balance, -18),
				Event:      model.ContractLifecycleDestroy,
			})
		case types.TRACE_REWARD:
			reward, _ := trace.Action.(types.Reward)
			if err := t.convertTrace(&dbTrace, "", reward.Author.Hex(), reward.Value, blockTime); err != nil {
				return newErrOperation(err)
			}
			dbTrace.ExtraField = "rewardType"
			dbTrace.ExtraValue = string(reward.RewardType)
		default:
			return newErrOperation(fmt.Errorf("unknown trace type: %v", trace.Type))
		}
		traceArr = append(traceArr, &dbTrace)
	}

	return &TraceOperation{
		dbBlock:   &dbBlock,
		minerAddr: &dbAddr,
		traceArr:  traceArr,
		txArr:     txArr,
	}
}

func buildTxFromReceipt(db *gorm.DB, receipts []*types.Receipt, blockTime time.Time) ([]*model.Tx, error) {
	var txArr []*model.Tx
	for _, receipt := range receipts {
		fromAddr, err := model.MakeAddrId(db, receipt.From.Hex(), blockTime)
		if err != nil {
			return nil, err
		}
		to := receipt.To
		if to == nil {
			to = receipt.ContractAddress
		}
		toAddr, err := model.MakeAddrId(db, to.Hex(), blockTime)
		if err != nil {
			return nil, err
		}
		tx := &model.Tx{
			FromId: fromAddr.ID,
			ToId:   toAddr.ID,
			Hash:   receipt.TransactionHash.Hex(),
			Status: uint(*receipt.Status),
		}
		txArr = append(txArr, tx)
	}
	return txArr, nil
}

func StartSync(ctx context.Context, wg *sync.WaitGroup, evmCfg *common.EvmConfig, db *gorm.DB) error {
	var utilCfg = evmCfg.Config
	processor := &TraceProcessor{
		db: db,
	}
	var dbBlock model.Block
	if err := db.Order("id desc").First(&dbBlock).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			dbBlock.ID = evmCfg.FirstBlock - 1
		} else {
			return errors.WithMessage(err, "Failed to get last block in DB")
		}
	}
	nextBlockNumber := dbBlock.ID + 1

	return evmUtil.StartFinalizedDB(ctx, wg, utilCfg, db, nextBlockNumber, processor)
}
