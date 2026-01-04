package cmd

import (
	"context"
	"github.com/Conflux-Chain/data-house/common"
	"github.com/Conflux-Chain/data-house/model"
	"github.com/Conflux-Chain/data-house/sync/evm"
	"github.com/Conflux-Chain/go-conflux-util/cmd"
	"github.com/Conflux-Chain/go-conflux-util/store"
	"github.com/Conflux-Chain/go-conflux-util/viper"
	"github.com/sirupsen/logrus"
	"sync"

	"github.com/spf13/cobra"
)

// startEvmSyncCmd represents the startEvmSync command
var startEvmSyncCmd = &cobra.Command{
	Use:   "startEvmSync",
	Short: "start evm sync",
	Run:   start,
}

func init() {
	rootCmd.AddCommand(startEvmSyncCmd)
}

func start(*cobra.Command, []string) {
	logrus.Info("Starting evm sync ...")

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	storeConfig := store.MustNewConfigFromViper()
	db := storeConfig.MustOpenOrCreate(model.EvmTables...)

	var config common.EvmConfig
	viper.MustUnmarshalKey("evm", &config)

	if err := evm.StartSync(ctx, &wg, &config, db); err != nil {
		logrus.WithError(err).Fatal("failed to start evm sync")
	}
	logrus.Info("Evm sync started")

	cmd.GracefulShutdown(&wg, cancel)
}
