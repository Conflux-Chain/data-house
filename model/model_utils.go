package model

import (
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

var (
	addrCache     = NewLRUCache[string, *Address](1024, time.Duration(0))
	topicCache    = NewLRUCache[string, *Topic](1024, time.Duration(0))
	logParamCache = NewLRUCache[string, *LogParam](1024, time.Duration(0))
)

func init() {

}

func MakeAddrId(tx *gorm.DB, addr string, blockTime time.Time) (*Address, error) {
	if bean, hit := addrCache.Get(addr); hit {
		return bean, nil
	}

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

	addrCache.Put(addr, &addrPre)

	return &addrPre, nil
}

func MakeTopicId(tx *gorm.DB, str string, blockTime time.Time) (*Topic, error) {
	if bean, hit := topicCache.Get(str); hit {
		return bean, nil
	}

	var topicPre Topic
	topicPre.Topic = str
	topicPre.CreatedAt = blockTime

	if str == "" {
		// topicPre's ID is 0
		return &topicPre, nil
	}
	if err := tx.Where("topic=?", str).Take(&topicPre).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err = tx.Create(&topicPre).Error; err != nil {
				return nil, errors.Wrap(err, "create topic")
			}
		} else {
			return nil, errors.Wrap(err, "query addr")
		}
	}

	topicCache.Put(str, &topicPre)

	return &topicPre, nil
}

func MakeLogParamId(tx *gorm.DB, str string, blockTime time.Time) (*LogParam, error) {
	if bean, hit := logParamCache.Get(str); hit {
		return bean, nil
	}

	var param LogParam
	param.Hex = str
	param.CreatedAt = blockTime

	if str == "" {
		// param's ID is 0
		return &param, nil
	}
	if err := tx.Where("Hex=?", str).Take(&param).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err = tx.Create(&param).Error; err != nil {
				return nil, errors.Wrap(err, "create log param")
			}
		} else {
			return nil, errors.Wrap(err, "query log param")
		}
	}

	logParamCache.Put(str, &param)

	return &param, nil
}

func PickParamId(param *LogParam) uint64 {
	if param == nil {
		return 0
	}
	return param.ID
}
