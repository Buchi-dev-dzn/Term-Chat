package protocol

import (
	"context"
	"testing"
	"time"
)

func TestRoomIsolation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, port, err := StartHost(ctx, []RoomConfig{
		{Name: "alpha", Passcode: "a", Protected: true},
		{Name: "beta", Passcode: "b", Protected: true},
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	addr := "127.0.0.1:" + itoa(port)
	alpha, err := Dial(ctx, addr, "alpha", "alice", "a")
	if err != nil {
		t.Fatalf("dial alpha: %v", err)
	}
	defer alpha.Close()

	beta, err := Dial(ctx, addr, "beta", "bob", "b")
	if err != nil {
		t.Fatalf("dial beta: %v", err)
	}
	defer beta.Close()

	alphaEvents := alpha.Receive(ctx)
	betaEvents := beta.Receive(ctx)

	if err := alpha.Send("hello alpha"); err != nil {
		t.Fatalf("send alpha: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-alphaEvents:
			if event.Kind == EventMessage && event.Body == "hello alpha" {
				return
			}
		case event := <-betaEvents:
			if event.Kind == EventMessage {
				t.Fatalf("beta leaked message: %#v", event)
			}
		case <-deadline:
			t.Fatal("timed out waiting for alpha message")
		}
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}

	var buf [16]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	return string(buf[i:])
}
