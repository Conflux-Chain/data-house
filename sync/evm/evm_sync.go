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
	dbBlock              *model.Block
	minerAddr            *model.Address
	traceArr             []*model.Trace
	txArr                []*model.Tx
	contractLifecycleArr []*model.ContractLifecycle
	logArr               []*model.Log
	Err                  error
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

	if len(o.contractLifecycleArr) > 0 {
		for _, lifecycle := range o.contractLifecycleArr {
			lifecycle.TxId = o.txArr[lifecycle.TransactionPosition].ID
		}
		if err := tx.Create(&o.contractLifecycleArr).Error; err != nil {
			return errors.Wrap(err, "create contract lifecycle")
		}
	}

	if len(o.logArr) > 0 {
		for _, log := range o.logArr {
			log.TxId = o.txArr[log.TxIndex].ID
		}
		if err := tx.Create(&o.logArr).Error; err != nil {
			return errors.Wrap(err, "create logs")
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

func (t *TraceProcessor) buildLogs(receipts []*types.Receipt, blockTime time.Time) ([]*model.Log, error) {
	var logArr []model.LogEntry
	logCounter := model.NewLogCounter()

	for _, receipt := range receipts {
		for _, log := range receipt.Logs {
			topicBean, err := model.MakeTopicId(t.db, log.Topics[0].Hex(), blockTime)
			if err != nil {
				return nil, errors.Wrap(err, "create topic bean")
			}
			addrBean, err := model.MakeAddrId(t.db, log.Address.Hex(), blockTime)
			if err != nil {
				return nil, errors.Wrap(err, "create contract bean")
			}

			logBean := &model.Log{
				BlockId:    log.BlockNumber,
				TopicId:    topicBean.ID,
				TxIndex:    log.TxIndex,
				ContractId: addrBean.ID,
				Model: model.Model{
					CreatedAt: blockTime,
				},
			}

			paramLen := len(log.Topics)
			errArr := make([]error, paramLen-1)
			switch paramLen {
			case 4:
				paramBean, err := model.MakeLogParamId(t.db, log.Topics[3].Hex(), blockTime)
				errArr[2] = err
				logBean.Param3 = model.PickParamId(paramBean)
				fallthrough
			case 3:
				paramBean, err := model.MakeLogParamId(t.db, log.Topics[2].Hex(), blockTime)
				errArr[1] = err
				logBean.Param2 = model.PickParamId(paramBean)
				fallthrough
			case 2:
				paramBean, err := model.MakeLogParamId(t.db, log.Topics[1].Hex(), blockTime)
				errArr[0] = err
				logBean.Param1 = model.PickParamId(paramBean)
			}
			for _, paramErr := range errArr {
				if paramErr != nil {
					return nil, errors.Wrap(paramErr, "make log param failed")
				}
			}

			// only keep the first log
			if cnt, key := logCounter.IncrementAndGet(logBean); cnt == 1 {
				logArr = append(logArr, model.LogEntry{
					Key:   key,
					Value: logBean,
				})
			}
		}
	}

	// set count
	var logs []*model.Log
	for _, entry := range logArr {
		log := entry.Value
		log.Count = logCounter.GetCount(entry.Key)
		logs = append(logs, log)
	}

	return logs, nil
}

func (t *TraceProcessor) Process(data evmUtil.BlockData) dbUtil.Operation {
	blockTime := time.Unix(int64(data.Block.Timestamp), 0)

	txArr, err := buildTxFromReceipt(t.db, data.Receipts, blockTime)
	if err != nil {
		return newErrOperation(errors.Wrap(err, "failed to build tx from receipts"))
	}

	logArr, err := t.buildLogs(data.Receipts, blockTime)
	if err != nil {
		return newErrOperation(errors.Wrap(err, "failed to build logs"))
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
				TransactionPosition: *trace.TransactionPosition,
				TxSenderId:          txArr[*trace.TransactionPosition].FromId,
				ContractId:          dbTrace.ToId,
				Value:               decimal.NewFromBigInt(create.Value, -18),
				Event:               model.ContractLifecycleCreate,
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
		dbBlock:              &dbBlock,
		minerAddr:            &dbAddr,
		traceArr:             traceArr,
		txArr:                txArr,
		contractLifecycleArr: contractLifecycleArr,
		logArr:               logArr,
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
			Model: model.Model{
				CreatedAt: blockTime,
			},
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
