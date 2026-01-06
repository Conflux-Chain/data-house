package cmd

import (
	"context"
	"sync"

	"github.com/Conflux-Chain/data-house/common"
	"github.com/Conflux-Chain/data-house/model"
	"github.com/Conflux-Chain/data-house/sync/cfx"
	"github.com/Conflux-Chain/go-conflux-util/cmd"
	"github.com/Conflux-Chain/go-conflux-util/store"
	"github.com/Conflux-Chain/go-conflux-util/viper"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// startCfxSyncCmd represents the startCfxSync command
var startCfxSyncCmd = &cobra.Command{
	Use:   "startCfxSync",
	Short: "start Cfx sync",
	Run:   startCfx,
}

func init() {
	rootCmd.AddCommand(startCfxSyncCmd)
}

func startCfx(*cobra.Command, []string) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	storeConfig := store.MustNewConfigFromViper()
	db := storeConfig.MustOpenOrCreate(model.CfxTables...)

	var config common.CfxConfig
	viper.MustUnmarshalKey("cfx", &config)

	logrus.WithField("batch", config.Batch).Info("Starting cfx sync ...")
	if config.Batch {
		if err := cfx.StartSyncBatch(ctx, &wg, &config, db); err != nil {
			logrus.Fatal(err)
		}
	} else {
		if err := cfx.StartSync(ctx, &wg, &config, db); err != nil {
			logrus.WithError(err).Fatal("failed to start cfx sync")
		}
	}
	logrus.Info("cfx sync started")

	cmd.GracefulShutdown(&wg, cancel)
}
