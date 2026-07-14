package tui

import (
	"fmt"
	"net"
	"slices"
	"strings"

	"termchat/internal/discovery"
	"termchat/internal/protocol"
)

type SetupMode string

const (
	ModeHost SetupMode = "host"
	ModeJoin SetupMode = "join"
)

type SetupState struct {
	Mode         SetupMode
	Nick         string
	Room         string
	Passcode     string
	Protected    bool
	SelectedPeer *discovery.DiscoveryRecord
	ManualAddr   string
}

func (s SetupState) TargetAddr() string {
	if s.SelectedPeer != nil {
		return net.JoinHostPort(s.SelectedPeer.Addr, fmt.Sprintf("%d", s.SelectedPeer.Port))
	}
	return strings.TrimSpace(s.ManualAddr)
}

func (s SetupState) Validate() error {
	if s.Mode != ModeHost && s.Mode != ModeJoin {
		return fmt.Errorf("mode is required")
	}
	if strings.TrimSpace(s.Nick) == "" {
		return fmt.Errorf("nickname is required")
	}
	if strings.TrimSpace(s.Room) == "" {
		return fmt.Errorf("room is required")
	}
	if s.Mode == ModeJoin && s.SelectedPeer == nil && strings.TrimSpace(s.ManualAddr) == "" {
		return fmt.Errorf("join target is required")
	}
	if s.Protected && strings.TrimSpace(s.Passcode) == "" {
		return fmt.Errorf("passcode is required for protected rooms")
	}
	return nil
}

func sortRecords(records []discovery.DiscoveryRecord) {
	slices.SortFunc(records, func(a, b discovery.DiscoveryRecord) int {
		if a.Room != b.Room {
			return strings.Compare(a.Room, b.Room)
		}
		return strings.Compare(a.Addr, b.Addr)
	})
}

func hostRoomConfig(state SetupState) protocol.RoomConfig {
	return protocol.RoomConfig{
		Name:      state.Room,
		Protected: state.Protected,
		Passcode:  state.Passcode,
	}
}

func localIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			return ip4.String()
		}
	}
	return "127.0.0.1"
}
