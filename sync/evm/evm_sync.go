package evm

import (
	"context"
	"sync"

	"github.com/Conflux-Chain/data-house/common"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func StartSync(ctx context.Context, wg *sync.WaitGroup, evmCfg *common.EvmConfig, db *gorm.DB) error {
	logrus.Info("starting sync")

	return nil
}
