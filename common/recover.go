package common

import (
	"github.com/sirupsen/logrus"
	"os"
)

func Recover() {
	if r := recover(); r != nil {
		logrus.Error("recover panic, ", r)
		os.Exit(-1)
	}
}
