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

func ReadBytes(c string) ([]byte, error) {
	return os.ReadFile(c)
}

func WriteBytes(c string, data []byte) error {
	return os.WriteFile(c, data, 0644)
}
