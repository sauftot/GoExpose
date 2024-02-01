package tests

import (
	"testing"
	"time"
)

func TestDone(t *testing.T) {
	done := make(chan struct{})
	timeout := time.After(1 * time.Second)
	res := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(done chan struct{}) {
			for {
				select {
				case <-done:
					res <- true
				default:
				}
			}
		}(done)
	}
	<-timeout
	t.Log("Sending done...")
	close(done)
	timeout = time.After(1 * time.Second)
	<-timeout
	if len(res) != 10 {
		t.Errorf("Expected 10, got %d", len(res))
	} else {
		t.Log("Received 10 results")
	}
}
