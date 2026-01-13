package model

import (
	"time"

	"github.com/shopspring/decimal"
)

var EvmTables = []any{
	&Address{}, &EvmBlock{}, &EvmTx{}, &EvmTrace{}, &ContractLifecycle{},
	&Topic{}, &LogParam{}, &EvmLog{},
}

var CfxTables = []any{
	&Address{}, &CfxBlock{}, &CfxTx{}, &CfxTrace{}, &ContractLifecycle{},
	&Topic{}, &LogParam{}, &CfxLog{},
}

type Model struct {
	ID        uint64    `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
	UpdatedAt time.Time `gorm:"not null" json:"updatedAt"`
}

type Address struct {
	Model
	// hex or non-verbose base32
	Address   string    `gorm:"size:64;unique;not null" json:"address"`
	BlockTime time.Time `gorm:"not null" json:"blockTime"`
}

type Block struct {
	Model
	Hash    string `gorm:"size:66;unique;not null" json:"hash"`
	MinerID uint64 `gorm:"not null" json:"minerId"`
	TxCount int    `gorm:"not null" json:"txCount"`
}

type EvmBlock struct {
	BlockNum uint64 `gorm:"not nul;unique" json:"blockNum"`
	Block
}

type CfxBlock struct {
	Epoch    uint64 `gorm:"not null;index:uk_epoch_position,unique" json:"epoch"`
	Position int    `gorm:"not null;index:uk_epoch_position,unique" json:"position"`
	Block
}

type BaseTx struct {
	Model
	Hash   string `gorm:"size:66;unique;not null" json:"hash"`
	FromId uint64 `gorm:"not null" json:"fromId"`
	ToId   uint64 `gorm:"not null" json:"toId"`
	Status uint   `gorm:"not null" json:"status"`
}

type EvmTx struct {
	BaseTx
	BlockNum uint64 `gorm:"not null;index:idx_block" json:"blockNum"`
}

type CfxTx struct {
	Epoch         uint64 `gorm:"not null;index:uk_ebt,unique" json:"epoch"`
	BlockPosition int8   `gorm:"not null;index:uk_ebt,unique" json:"blockPosition"`
	TxPosition    int8   `gorm:"not null;index:uk_ebt,unique" json:"txPosition"`
	BaseTx
}

type Trace struct {
	Model
	TxId                uint64 `gorm:"not null" json:"txId"`
	TransactionPosition uint   `gorm:"not null" json:"transactionPosition"`
	FromId              uint64 `gorm:"not null" json:"fromId"`
	ToId                uint64 `gorm:"not null" json:"toId"`
	TraceType           string `gorm:"type:varchar(32);not null" json:"traceType"`
	// for different actions, these two fields could be:
	// callType, createType, refundAddress, rewardType
	ExtraField string `gorm:"size:16;not null" json:"extraField"`
	ExtraValue string `gorm:"size:64;not null" json:"extraValue"`

	Valid      bool            `gorm:"not null" json:"valid"`
	Value      decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"value"`
	Method     string          `gorm:"size:10;not null" json:"method"`
	TraceIndex int             `gorm:"not null" json:"traceIndex"`
}

type EvmTrace struct {
	BlockNum uint64 `gorm:"not null;index:idx_block" json:"blockNum"`
	Trace
}

type CfxTrace struct {
	Trace
	Epoch      uint64 `gorm:"not null:index:idx_epoch" json:"epoch"`
	FromPocket string `gorm:"size:64;not null" json:"fromPocket"`
	ToPocket   string `gorm:"size:64;not null" json:"toPocket"`
	FromSpace  string `gorm:"size:64;not null" json:"fromSpace"`
	ToSpace    string `gorm:"size:64;not null" json:"toSpace"`
	Outcome    string `gorm:"type:varchar(16);not null" json:"outcome"`
}

type ContractLifecycle struct {
	Model
	TxId                uint64          `gorm:"not null" json:"txId"`
	TransactionPosition uint            `json:"transactionPosition"`
	TxSenderId          uint64          `gorm:"not null" json:"txSenderId"`
	ContractId          uint64          `gorm:"not null" json:"contractId"`
	Event               string          `gorm:"not null" json:"event"`
	RefundAddrId        uint64          `gorm:"not null" json:"refundAddrId"`
	Value               decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"value"`
}

type Topic struct {
	Model
	Topic string `gorm:"size:66;not null;unique" json:"topic"`
}
type LogParam struct {
	Model
	Hex string `gorm:"size:66;not null;unique" json:"topic"`
}

type Log struct {
	Model
	TxIndex    uint   `gorm:"not null" json:"txIndex"`
	TxId       uint64 `gorm:"not null" json:"txId"`
	ContractId uint64 `gorm:"not null" json:"contractId"`
	TopicId    uint64 `gorm:"not null" json:"topicId"`
	// refer to LogParam
	Param1 uint64 `gorm:"not null" json:"param1"`
	Param2 uint64 `gorm:"not null" json:"param2"`
	Param3 uint64 `gorm:"not null" json:"param3"`
	Count  uint   `gorm:"not null" json:"count"`
}

type EvmLog struct {
	BlockNum uint64 `gorm:"not null" json:"blockNum"`
	Log
}

type CfxLog struct {
	Epoch         uint64 `gorm:"not null;index:idx_epoch" json:"epoch"`
	BlockPosition int8   `gorm:"not null" json:"blockPosition"`
	Log
}

const (
	ContractLifecycleCreate  = "create"
	ContractLifecycleDestroy = "destroy"
)
