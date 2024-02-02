package tests

import "testing"
import log "example.com/reverseproxy/pkg/logger"

func TestLogger(t *testing.T) {
	logger, err := log.NewLogger("test")
	if err != nil {
		t.Error(err)
	}
	logger.SetLogLevel(log.DEBUG)
	logger.Log("Test DEBUG Message")
	logger.SetLogLevel(log.INFO)
	logger.Log("Test INFO Message")
}
