package cmd

import (
	"context"
	"fmt"
	"github.com/Conflux-Chain/data-house/model"
	"github.com/Conflux-Chain/go-conflux-util/api"
	"github.com/Conflux-Chain/go-conflux-util/api/middleware"
	"github.com/Conflux-Chain/go-conflux-util/cmd"
	"github.com/Conflux-Chain/go-conflux-util/store"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"sync"
	"time"
)

// startMonitorCmd represents the startMonitor command
var startMonitorCmd = &cobra.Command{
	Use: "startMonitor",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("startMonitor called")
		startMonitor()
	},
}

var db *gorm.DB

func startMonitor() {
	_, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	storeConfig := store.MustNewConfigFromViper()

	db = storeConfig.MustOpenOrCreate(&model.KV{})

	factory := func(router *gin.Engine) {
		router.GET("/", middleware.Wrap(func(c *gin.Context) (any, error) {
			return "data house api", nil
		}))

		grp := router.Group("/dh")
		grp.GET("/progress", middleware.Wrap(progress))
	}
	api.MustServeFromViper(factory)

	cmd.GracefulShutdown(&wg, cancel)
}

func progress(c *gin.Context) (any, error) {
	return getSyncProgress(db)
}

type ChainLatestBlock struct {
	ChainID   string    `json:"chain_id"`
	BlockNum  uint64    `json:"block_num"`
	CreatedAt time.Time `json:"created_at"`
}

func getSyncProgress(db *gorm.DB) ([]ChainLatestBlock, error) {
	sql := `
(SELECT '1029' as chain_id, epoch as block_num, created_at FROM data_1029.cfx_blocks ORDER BY epoch DESC LIMIT 1) UNION
(SELECT '1030' as chain_id, block_num, created_at FROM data_1030.evm_blocks ORDER BY block_num DESC LIMIT 1) UNION
(SELECT 'net1' as chain_id, epoch as block_num, created_at FROM data_net1.cfx_blocks ORDER BY epoch DESC LIMIT 1 )UNION
(SELECT 'net71' as chain_id, block_num, created_at FROM data_net71.evm_blocks ORDER BY block_num DESC LIMIT 1)
`

	var results []ChainLatestBlock
	err := db.Raw(sql).Scan(&results).Error
	if err != nil {
		return nil, errors.Wrap(err, "get sync progress")
	}

	return results, nil
}

func init() {
	rootCmd.AddCommand(startMonitorCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// startMonitorCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// startMonitorCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
