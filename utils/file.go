package utils

import (
	log "github.com/sirupsen/logrus"
	"os"
)

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	log.Infof("unable to stat path %q; %v", path, err)
	return false
}
