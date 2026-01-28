package cfx

import (
	"context"
	"fmt"
	sdk "github.com/Conflux-Chain/go-conflux-sdk"
	"github.com/sirupsen/logrus"
	"math/big"
	"sync"
	"time"

	"github.com/Conflux-Chain/data-house/common"
	"github.com/Conflux-Chain/data-house/model"
	"github.com/Conflux-Chain/go-conflux-sdk/types"
	syncUtil "github.com/Conflux-Chain/go-conflux-util/blockchain/sync"
	coreUtil "github.com/Conflux-Chain/go-conflux-util/blockchain/sync/core"
	dbUtil "github.com/Conflux-Chain/go-conflux-util/blockchain/sync/process/db"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var (
	TxStatusSuccess = uint64(0)
	TxStatusSkip    = uint64(2)
)

type TraceProcessor struct {
	db *gorm.DB
}

var lastSavepoint uint64

type TraceOperation struct {
	dbBlock              []model.CfxBlock
	traceArr             []*model.CfxTrace
	txArr                []*model.CfxTx
	txArr2d              [][]*model.CfxTx
	contractLifecycleArr []*model.ContractLifecycle
	logArr               []*model.CfxLog
	Err                  error
}

func newErrOperation(err error) *TraceOperation {
	return &TraceOperation{
		Err: err,
	}
}

func (o *TraceOperation) Exec(tx *gorm.DB) error {
	defer common.Recover()

	if o.Err != nil {
		logrus.WithError(o.Err).Error("Failed to execute operation")
		return o.Err
	}

	curEpoch := o.dbBlock[0].Epoch
	if curEpoch != lastSavepoint+1 {
		msg := fmt.Sprintf("excpect epoch %d, got %d", lastSavepoint+1, curEpoch)
		logrus.Error(msg)
		return fmt.Errorf(msg)
	}

	//save block
	if err := tx.Create(&o.dbBlock).Error; err != nil {
		return errors.Wrap(err, "create block failed")
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

	batchSize := 1000
	if len(o.txArr) > 0 {
		if err := tx.CreateInBatches(&o.traceArr, batchSize).Error; err != nil {
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
			blockTx := o.txArr2d[log.BlockPosition]
			log.TxId = blockTx[log.TxIndex].ID
		}
		if err := tx.CreateInBatches(&o.logArr, batchSize).Error; err != nil {
			return errors.Wrap(err, "create logs")
		}
	}

	lastSavepoint = curEpoch

	if curEpoch%1000 == 0 {
		logrus.WithFields(logrus.Fields{
			"epoch": curEpoch,
		}).Info("save epoch")
	}

	return nil
}

func (t *TraceProcessor) convertTrace(dbTrace *model.CfxTrace, from string, to string,
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
	dbTrace.Model.CreatedAt = blockTime

	return nil
}

func (t *TraceProcessor) buildLogsArr(receipts [][]types.TransactionReceipt, blockTime time.Time) ([]*model.CfxLog, error) {
	var logs []*model.CfxLog

	for index, receipt := range receipts {
		arr, err := t.buildLogs(receipt, blockTime, index)
		if err != nil {
			return nil, err
		}
		logs = append(logs, arr...)
	}

	return logs, nil
}

func (t *TraceProcessor) buildLogs(receipts []types.TransactionReceipt, blockTime time.Time, blockIndex int) ([]*model.CfxLog, error) {
	var logArr []model.LogEntry
	logCounter := model.NewLogCounter()

	txIdx := -1
	for _, receipt := range receipts {
		if uint64(receipt.OutcomeStatus) == TxStatusSkip {
			continue
		}
		txIdx++
		for _, log := range receipt.Logs {
			topicBean, err := model.MakeTopicId(t.db, log.Topics[0].String(), blockTime)
			if err != nil {
				return nil, errors.Wrap(err, "create topic bean")
			}
			addrBean, err := model.MakeAddrId(t.db, log.Address.String(), blockTime)
			if err != nil {
				return nil, errors.Wrap(err, "create contract bean")
			}

			logBean := &model.Log{
				TopicId:    topicBean.ID,
				TxIndex:    uint(txIdx),
				ContractId: addrBean.ID,
				Model: model.Model{
					ID:        uint64(receipt.EpochNumber),
					CreatedAt: blockTime,
				},
			}

			paramLen := len(log.Topics)
			errArr := make([]error, paramLen-1)
			switch paramLen {
			case 4:
				paramBean, err := model.MakeLogParamId(t.db, log.Topics[3].String(), blockTime)
				errArr[2] = err
				logBean.Param3 = model.PickParamId(paramBean)
				fallthrough
			case 3:
				paramBean, err := model.MakeLogParamId(t.db, log.Topics[2].String(), blockTime)
				errArr[1] = err
				logBean.Param2 = model.PickParamId(paramBean)
				fallthrough
			case 2:
				paramBean, err := model.MakeLogParamId(t.db, log.Topics[1].String(), blockTime)
				errArr[0] = err
				logBean.Param1 = model.PickParamId(paramBean)
			}
			for _, paramErr := range errArr {
				if paramErr != nil {
					return nil, errors.Wrap(paramErr, "make log param failed")
				}
			}

			// only keep the first log
			if cnt, key := logCounter.IncrementAndGet(logBean, uint64(blockIndex)); cnt == 1 {
				logArr = append(logArr, model.LogEntry{
					Key:   key,
					Value: logBean,
				})
			}
		}
	}

	// set count
	var logs []*model.CfxLog
	for _, entry := range logArr {
		log := entry.Value
		log.Count = logCounter.GetCount(entry.Key)
		cfxLog := &model.CfxLog{
			Epoch:         log.ID,
			BlockPosition: int16(blockIndex),
			Log:           *log,
		}
		cfxLog.ID = 0
		logs = append(logs, cfxLog)

	}

	return logs, nil
}

type BlockRef struct {
	block *types.Block
	idx   int
}

func buildBlockMap(blocks []*types.Block) map[string]BlockRef {
	blockMap := make(map[string]BlockRef)
	for idx, block := range blocks {
		blockMap[block.Hash.String()] = BlockRef{
			block: block,
			idx:   idx,
		}
	}
	return blockMap
}

func (t *TraceProcessor) Process(data coreUtil.EpochData) dbUtil.Operation {
	defer common.Recover()

	epoch := data.Blocks[0].EpochNumber.ToInt().Uint64()
	if epoch%1000 == 0 {
		logrus.WithFields(logrus.Fields{
			"epoch": epoch,
		}).Info("Processing Epoch Data")
	}

	blockCount := len(data.Blocks)
	pivotBlock := data.Blocks[blockCount-1]
	blockTime := time.Unix(pivotBlock.Timestamp.ToInt().Int64(), 0)
	blockMap := buildBlockMap(data.Blocks)

	txArr, blockTxInfo, err := buildTxFromReceiptArr(t.db, data.Receipts, blockTime)
	if err != nil {
		return newErrOperation(errors.Wrap(err, "failed to build tx from receipts"))
	}

	cfxLogArr, err := t.buildLogsArr(data.Receipts, blockTime)
	if err != nil {
		return newErrOperation(errors.Wrap(err, "failed to build logs"))
	}

	var blockArr []model.CfxBlock
	for idx, block := range data.Blocks {
		addrId, err := model.MakeAddrId(t.db, block.Miner.String(), blockTime)
		if err != nil {
			return newErrOperation(errors.Wrap(err, "failed to make addr id"))
		}
		dbBlock := model.Block{
			Model: model.Model{
				ID:        0,
				CreatedAt: blockTime,
			},
			TxCount: len(blockTxInfo[idx]),
			Hash:    block.Hash.String(),
			MinerID: addrId.ID,
		}

		cfxBlock := model.CfxBlock{
			Epoch:    block.EpochNumber.ToInt().Uint64(),
			Block:    dbBlock,
			Position: int16(idx),
		}
		blockArr = append(blockArr, cfxBlock)
	}

	//
	type TxInfo struct {
		idx    uint
		fromId uint64
	}
	txMap := make(map[string]TxInfo)
	for idx, tx := range txArr {
		txMap[tx.Hash] = TxInfo{uint(idx), tx.FromId}
	}
	// parse trace
	var traceArr []*model.CfxTrace
	var contractLifecycleArr []*model.ContractLifecycle
	var creationStack = common.NewStack[types.Create]()
	var callStack = common.NewStack[*model.CfxTrace]()
	for index, trace := range data.Traces.CfxTraces {
		blockRef := blockMap[trace.BlockHash.String()]
		txRef := data.Receipts[blockRef.idx][int(trace.TransactionPosition)]

		dbTrace := model.CfxTrace{
			Epoch: epoch,
			Trace: model.Trace{
				TraceIndex:          index,
				TraceType:           string(trace.Type),
				Valid:               trace.Valid,
				TransactionPosition: txMap[trace.TransactionHash.String()].idx,
			},
		}
		addTrace := true
		switch trace.Type {
		case types.TRACE_CALL:
			call, _ := trace.Action.(types.Call)
			if err := t.convertTrace(&dbTrace, call.From.String(), call.To.String(), call.Value.ToInt(), blockTime); err != nil {
				return newErrOperation(err)
			}
			dbTrace.ExtraField = "callType"
			dbTrace.ExtraValue = string(call.CallType)
			dbTrace.Method = common.GetMethodID(call.Input)
			dbTrace.ToSpace = string(call.Space)
			dbTrace.FromSpace = string(call.Space)
			callStack.Push(&dbTrace)
		case types.TRACE_CALL_RESULT:
			cr, _ := trace.Action.(types.CallResult)
			callTrace, _ := callStack.Pop()
			callTrace.Outcome = string(cr.Outcome)
			addTrace = false
		case types.TRACE_CREATE:
			create, _ := trace.Action.(types.Create)
			creationStack.Push(create)
		case types.TRACE_CREATE_RESULT:
			create, _ := creationStack.Pop()
			result, _ := trace.Action.(types.CreateResult)
			if err := t.convertTrace(&dbTrace, txRef.From.String(), result.Addr.String(), create.Value.ToInt(), blockTime); err != nil {
				return newErrOperation(err)
			}
			dbTrace.ExtraField = "createType"
			dbTrace.ExtraValue = string(create.CreateType)
			contractLifecycleArr = append(contractLifecycleArr, &model.ContractLifecycle{
				Model: model.Model{
					CreatedAt: blockTime,
				},
				TransactionPosition: txMap[trace.TransactionHash.String()].idx,
				TxSenderId:          txMap[trace.TransactionHash.String()].fromId,
				ContractId:          dbTrace.ToId,
				Value:               decimal.NewFromBigInt(create.Value.ToInt(), -18),
				Event:               model.ContractLifecycleCreate,
			})
			addTrace = false
		case types.TRACE_INTERNAL_TRANSFER_ACTIION:
			ita, _ := trace.Action.(types.InternalTransferAction)
			if err := t.convertTrace(&dbTrace, ita.From.String(), ita.To.String(), ita.Value.ToInt(), blockTime); err != nil {
				return newErrOperation(err)
			}
			dbTrace.FromPocket = string(ita.FromPocket)
			dbTrace.ToPocket = string(ita.ToPocket)
			dbTrace.ToSpace = string(ita.ToSpace)
			dbTrace.FromSpace = string(ita.FromSpace)
		case types.TRACE_SELF_DESTRUCT:
			sd, _ := trace.Action.(types.SelfDestruct)
			if err := t.convertTrace(&dbTrace, sd.Address.String(), sd.RefundAddress.String(), sd.Balance.ToInt(), blockTime); err != nil {
				return newErrOperation(err)
			}

			dbTrace.ExtraField = "refundAddress"
			dbTrace.ExtraValue = sd.RefundAddress.String()

			contractLifecycleArr = append(contractLifecycleArr, &model.ContractLifecycle{
				Model: model.Model{
					CreatedAt: blockTime,
				},
				TransactionPosition: txMap[trace.TransactionHash.String()].idx,
				TxSenderId:          txMap[trace.TransactionHash.String()].fromId,
				ContractId:          dbTrace.ToId,
				Value:               decimal.NewFromBigInt(sd.Balance.ToInt(), -18),
				Event:               model.ContractLifecycleDestroy,
			})
		default:
			return newErrOperation(fmt.Errorf("unknown trace type: %v", trace.Type))
		}
		if addTrace {
			traceArr = append(traceArr, &dbTrace)
		}
	}

	if creationStack.Size() > 0 {
		return newErrOperation(fmt.Errorf("contract creation stack overflow"))
	}
	if callStack.Size() > 0 {
		return newErrOperation(fmt.Errorf("trace call stack overflow"))
	}

	return &TraceOperation{
		dbBlock:              blockArr,
		traceArr:             traceArr,
		txArr:                txArr,
		txArr2d:              blockTxInfo,
		contractLifecycleArr: contractLifecycleArr,
		logArr:               cfxLogArr,
	}
}

func buildTxFromReceiptArr(db *gorm.DB, receipts [][]types.TransactionReceipt, blockTime time.Time) ([]*model.CfxTx, [][]*model.CfxTx, error) {
	var txs []*model.CfxTx
	var tx2d = make([][]*model.CfxTx, len(receipts))
	var blockTxInfo = make([]int, len(receipts))
	for idx, receipt := range receipts {
		arr, err := buildTxFromReceipt(db, receipt, int16(idx), blockTime)
		if err != nil {
			return nil, nil, err
		}
		txs = append(txs, arr...)
		blockTxInfo[idx] = len(arr)
		tx2d[idx] = arr
	}
	return txs, tx2d, nil
}
func buildTxFromReceipt(db *gorm.DB, receipts []types.TransactionReceipt, blockPos int16, blockTime time.Time) ([]*model.CfxTx, error) {
	var txArr []*model.CfxTx
	for txIdx, receipt := range receipts {
		status := uint64(receipt.OutcomeStatus)
		if status == TxStatusSkip {
			continue
		}
		fromAddr, err := model.MakeAddrId(db, receipt.From.String(), blockTime)
		if err != nil {
			return nil, err
		}

		to := receipt.To
		failedToCreate := false
		if to == nil {
			to = receipt.ContractCreated
			if to == nil && status != TxStatusSuccess {
				failedToCreate = true
			}
		}
		toHex := ""
		if failedToCreate {
			// it's ok
		} else if to == nil {
			return nil, fmt.Errorf("invalid receipt address, block %v, tx %v, field %v %v",
				receipt.BlockHash.String(), receipt.TransactionHash.String(), receipt.To, receipt.ContractCreated)
		} else {
			toHex = to.String()
		}
		toAddr, err := model.MakeAddrId(db, toHex, blockTime)
		if err != nil {
			return nil, err
		}

		baseTx := model.BaseTx{
			Model: model.Model{
				CreatedAt: blockTime,
			},
			FromId: fromAddr.ID,
			ToId:   toAddr.ID,
			Hash:   receipt.TransactionHash.String(),
			Status: uint(status),
		}
		cfxTx := &model.CfxTx{
			Epoch:         uint64(receipt.EpochNumber),
			BlockPosition: blockPos,
			TxPosition:    int16(txIdx),
			BaseTx:        baseTx,
		}
		txArr = append(txArr, cfxTx)
	}
	return txArr, nil
}

func StartSync(ctx context.Context, wg *sync.WaitGroup, cfxCfg *common.CfxConfig, db *gorm.DB) error {
	var paramsDB = cfxCfg.ParamsDB
	processor := &TraceProcessor{
		db: db,
	}
	nextBlockNumber, err := calculateNextBlockNo(db, cfxCfg)
	if err != nil {
		return err
	}
	paramsDB.DB = db
	adapter, errAd := coreUtil.NewAdapterWithConfig(cfxCfg.AdapterConfig)
	if errAd != nil {
		return errAd
	}
	cfxClient, err := sdk.NewClient(cfxCfg.AdapterConfig.URL, sdk.ClientOption{})
	if err != nil {
		return err
	}
	logAdapter := common.NewLoggingAdapter[coreUtil.EpochData](adapter, nil, "coreSync")
	logAdapter.CfxClient = cfxClient
	paramsDB.Adapter = logAdapter

	if nextBlockNumber == 0 {
		processor.prepareEpoch0(ctx, cfxCfg)
		nextBlockNumber = 1
	}

	logrus.Info("next block number: ", nextBlockNumber)
	paramsDB.NextBlockNumber = nextBlockNumber
	lastSavepoint = nextBlockNumber - 1
	syncUtil.StartFinalizedDB(ctx, wg, paramsDB, processor)

	return nil
}

func (t *TraceProcessor) prepareEpoch0(ctx context.Context, cfxCfg *common.CfxConfig) {
	if cfxCfg.AdapterConfig.AdapterOption.IgnoreTraces {
		return
	}
	cfxCfg.AdapterConfig.AdapterOption.IgnoreTraces = true
	adapter, errAd := coreUtil.NewAdapterWithConfig(cfxCfg.AdapterConfig)
	if errAd != nil {
		logrus.Fatal("failed to init adapter", errAd)
	}
	data, err := adapter.GetBlockData(ctx, 0)
	if err != nil {
		logrus.Fatal("failed to get block data", err)
	}
	data.Traces = &types.EpochTrace{
		CfxTraces:        nil,
		EthTraces:        nil,
		MirrorAddressMap: nil,
	}

	if len(data.Receipts) == 0 {
		logrus.Warn("no receipts found at epoch 0")
		return
	}

	op := t.Process(data)
	if err := op.Exec(t.db); err != nil {
		logrus.Fatal(err)
	}

	cfxCfg.AdapterConfig.AdapterOption.IgnoreTraces = false
}

func calculateNextBlockNo(db *gorm.DB, cfxCfg *common.CfxConfig) (uint64, error) {
	var dbBlock model.CfxBlock
	if err := db.Order("id desc").First(&dbBlock).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cfxCfg.FirstBlock, nil
		} else {
			return 0, errors.WithMessage(err, "Failed to get last block in DB")
		}
	}
	nextBlockNumber := dbBlock.Epoch + 1
	return nextBlockNumber, nil
}

type BatchProcessor struct {
	TraceProcessor
	TraceOpArr  []*TraceOperation
	StopAtBlock uint64
	Stopped     bool
}

func (p *BatchProcessor) BatchProcess(data coreUtil.EpochData) int {
	failFastFlag := 90_0000
	if data.Blocks[0].EpochNumber.ToInt().Uint64() > p.StopAtBlock {
		p.Stopped = true
		return failFastFlag
	}

	dbOp := p.Process(data).(*TraceOperation)
	if dbOp.Err != nil {
		return failFastFlag // fulfill batch and fail fast
	}
	p.TraceOpArr = append(p.TraceOpArr, dbOp)
	beanCount := 1 +
		len(dbOp.contractLifecycleArr) +
		len(dbOp.txArr) +
		len(dbOp.logArr) +
		len(dbOp.traceArr)

	return beanCount
}

func (p *BatchProcessor) BatchExec(tx *gorm.DB, _ int) error {
	if p.Stopped && len(p.TraceOpArr) == 0 {
		return fmt.Errorf("batch processor stopped at block %d", p.StopAtBlock)
	}
	for _, op := range p.TraceOpArr {
		if err := op.Exec(tx); err != nil {
			return err
		}
	}

	return nil
}

func (p *BatchProcessor) BatchReset() {
	p.TraceOpArr = nil
}

func StartSyncBatch(ctx context.Context, _ *sync.WaitGroup, cfxCfg *common.CfxConfig, db *gorm.DB) error {
	processor := &BatchProcessor{
		TraceProcessor: TraceProcessor{
			db: db,
		},
	}
	nextBlockNumber, err := calculateNextBlockNo(db, cfxCfg)
	if err != nil {
		return err
	}
	cfxCfg.CatchupParamsDB.NextBlockNumber = nextBlockNumber

	adapter, errAd := coreUtil.NewAdapterWithConfig(cfxCfg.AdapterConfig)
	if errAd != nil {
		return errAd
	}
	logAdapter := common.NewLoggingAdapter[coreUtil.EpochData](adapter, nil, "coreSync")

	finalizedBlock, errF := adapter.GetFinalizedBlockNumber(ctx)
	if errF != nil {
		return errF
	}
	processor.StopAtBlock = finalizedBlock
	cfxCfg.CatchupParamsDB.Adapter = logAdapter
	cfxCfg.CatchupParamsDB.DB = db

	syncUtil.CatchUpDB(ctx, cfxCfg.CatchupParamsDB, processor)

	return nil
}
