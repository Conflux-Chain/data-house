package common

import (
	"github.com/sirupsen/logrus"
	"os"
	"runtime/debug"
)

func Recover() {
	if r := recover(); r != nil {
		logrus.Error("recover panic, ", r)
		logrus.Info(string(debug.Stack()))
		os.Exit(-1)
	}
}
