package tui

import (
	"testing"

	"termchat/internal/discovery"
)

func TestSetupValidateProtectedJoinNeedsTarget(t *testing.T) {
	state := SetupState{
		Mode:      ModeJoin,
		Nick:      "alice",
		Room:      "lobby",
		Protected: true,
		Passcode:  "secret",
	}
	if err := state.Validate(); err == nil {
		t.Fatal("expected join target error")
	}
}

func TestSetupValidateOpenRoomNoPasscode(t *testing.T) {
	state := SetupState{
		Mode:       ModeJoin,
		Nick:       "alice",
		Room:       "lobby",
		Protected:  false,
		ManualAddr: "127.0.0.1:9000",
	}
	if err := state.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestTargetAddrFromSelectedPeer(t *testing.T) {
	state := SetupState{
		SelectedPeer: &discovery.DiscoveryRecord{
			Addr: "192.168.0.10",
			Port: 9999,
		},
	}
	if got := state.TargetAddr(); got != "192.168.0.10:9999" {
		t.Fatalf("addr=%q", got)
	}
}
