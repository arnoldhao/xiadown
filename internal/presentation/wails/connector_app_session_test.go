package wails

import (
	"sync"
	"testing"
)

func TestConnectorAppSessionCompleteCloseIsSingleTransition(t *testing.T) {
	session := &connectorAppSessionWindow{done: make(chan struct{})}
	start := make(chan struct{})
	var wait sync.WaitGroup
	for i := 0; i < 32; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			session.completeClose(nil)
		}()
	}
	close(start)
	wait.Wait()

	select {
	case <-session.done:
	default:
		t.Fatal("expected done channel to be closed")
	}
}
