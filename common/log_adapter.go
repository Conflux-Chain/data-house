package common

import (
	"context"
	sdk "github.com/Conflux-Chain/go-conflux-sdk"
	"github.com/Conflux-Chain/go-conflux-util/blockchain/sync/poll"
	"github.com/sirupsen/logrus"
	"strings"
)

// LoggingAdapter is a wrapper that adds structured logging to InnerAdapter.
type LoggingAdapter[T any] struct {
	// inner is the wrapped adapter implementation
	inner poll.Adapter[T]
	// logger is the structured logger instance
	logger *logrus.Logger
	// component identifies the logging component
	component string
	CfxClient *sdk.Client
}

// NewLoggingAdapter creates a new LoggingAdapter with the given inner adapter.
// If logger is nil, a new logrus logger is created with default settings.
func NewLoggingAdapter[T any](inner poll.Adapter[T], logger *logrus.Logger, component string) *LoggingAdapter[T] {
	if logger == nil {
		logger = logrus.New()
	}
	if component == "" {
		component = "adapter"
	}

	return &LoggingAdapter[T]{
		inner:     inner,
		logger:    logger,
		component: component,
	}
}

// GetFinalizedBlockNumber returns the finalized block number, logging any errors.
func (a *LoggingAdapter[T]) GetFinalizedBlockNumber(ctx context.Context) (uint64, error) {
	logEntry := a.logger.WithFields(logrus.Fields{
		"component": a.component,
		"operation": "GetFinalizedBlockNumber",
	})

	blockNumber, err := a.inner.GetFinalizedBlockNumber(ctx)
	if err != nil {
		logEntry.WithError(err).Error("Failed to get finalized block number")
	}

	return blockNumber, err
}

// GetLatestBlockNumber returns the latest block number, logging any errors.
func (a *LoggingAdapter[T]) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	logEntry := a.logger.WithFields(logrus.Fields{
		"component": a.component,
		"operation": "GetLatestBlockNumber",
	})

	blockNumber, err := a.inner.GetLatestBlockNumber(ctx)
	if err != nil {
		logEntry.WithError(err).Error("Failed to get latest block number")
	}

	return blockNumber, err
}

// GetBlockData returns the whole blockchain data of the given block number, logging any errors.
func (a *LoggingAdapter[T]) GetBlockData(ctx context.Context, blockNumber uint64) (T, error) {
	logEntry := a.logger.WithFields(logrus.Fields{
		"component":    a.component,
		"operation":    "GetBlockData",
		"block_number": blockNumber,
	})

	data, err := a.inner.GetBlockData(ctx, blockNumber)
	if err != nil {
		strErr := err.Error()
		if strings.HasSuffix(strErr, "timeout") {
			//that's fine
		} else {
			logrus.WithError(err).Error("Failed to get block data ", blockNumber)
			logEntry.WithError(err).WithFields(logrus.Fields{
				"block_number": blockNumber,
			}).Error("Failed to get block data")
		}
	}

	return data, err
}

// GetBlockHash returns the block hash of given blockchain data.
func (a *LoggingAdapter[T]) GetBlockHash(data T) string {
	return a.inner.GetBlockHash(data)
}

// GetParentBlockHash returns the parent block hash of given blockchain data.
func (a *LoggingAdapter[T]) GetParentBlockHash(data T) string {
	return a.inner.GetParentBlockHash(data)
}
