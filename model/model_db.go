package model

import (
	"github.com/openweb3/web3go/types"
	"github.com/shopspring/decimal"
	"time"
)

var EvmTables = []any{
	&Address{}, &Block{}, &Tx{}, &Trace{}, &ContractLifecycle{},
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

type Tx struct {
	Model
	BlockId uint64 `gorm:"not null" json:"blockId"`
	Hash    string `gorm:"size:66;unique;not null" json:"hash"`
	FromId  uint64 `gorm:"not null" json:"fromId"`
	ToId    uint64 `gorm:"not null" json:"toId"`
	Status  uint   `gorm:"not null" json:"status"`
}

type Trace struct {
	Model
	TxId                uint64          `gorm:"not null" json:"txId"`
	TransactionPosition uint            `gorm:"not null" json:"transactionPosition"`
	FromId              uint64          `gorm:"not null" json:"fromId"`
	ToId                uint64          `gorm:"not null" json:"toId"`
	TraceType           types.TraceType `gorm:"type:varchar(16);not null" json:"traceType"`
	// for different actions, these two fields could be:
	// callType, createType, refundAddress, rewardType
	ExtraField string `gorm:"size:16;not null" json:"extraField"`
	ExtraValue string `gorm:"size:64;not null" json:"extraValue"`

	Valid      bool            `gorm:"not null" json:"valid"`
	Value      decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"value"`
	Method     string          `gorm:"size:10;not null" json:"method"`
	TraceIndex int             `gorm:"not null" json:"traceIndex"`
}

type ContractLifecycle struct {
	Model
	TxId         uint64          `gorm:"not null" json:"txId"`
	TxSenderId   uint64          `gorm:"not null" json:"txSenderId"`
	ContractId   uint64          `gorm:"not null" json:"contractId"`
	Event        string          `gorm:"not null" json:"event"`
	RefundAddrId uint64          `gorm:"not null" json:"refundAddrId"`
	Value        decimal.Decimal `gorm:"type:decimal(36,18);not null" json:"value"`
}

const (
	ContractLifecycleCreate  = "create"
	ContractLifecycleDestroy = "destroy"
)
