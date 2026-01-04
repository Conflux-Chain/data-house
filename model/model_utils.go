package model

import (
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func MakeAddrId(tx *gorm.DB, addr string, blockTime time.Time) (*Address, error) {
	//TODO K add cache
	var addrPre Address
	addrPre.Address = addr
	addrPre.BlockTime = blockTime

	if addr == "" {
		// addrPre's ID is 0
		return &addrPre, nil
	}
	if err := tx.Where("address=?", addr).Take(&addrPre).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err = tx.Create(&addrPre).Error; err != nil {
				return nil, errors.Wrap(err, "create addr")
			}
		} else {
			return nil, errors.Wrap(err, "query addr")
		}
	}

	return &addrPre, nil
}
