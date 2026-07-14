package tui

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

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

func runSetup(ctx context.Context, ui *lineUI) (SetupState, error) {
	var state SetupState

	fmt.Fprintln(ui.out, "termchat")
	fmt.Fprintln(ui.out, "1 command. LAN rooms.")
	fmt.Fprintln(ui.out)

	mode, err := ui.askChoice("1. Host a room\n2. Join a room\n", []string{"1", "2"})
	if err != nil {
		return state, err
	}
	if mode == "1" {
		state.Mode = ModeHost
	} else {
		state.Mode = ModeJoin
	}

	nick, err := ui.ask("Nickname")
	if err != nil {
		return state, err
	}
	state.Nick = nick

	if state.Mode == ModeHost {
		if err := fillHostSetup(ui, &state); err != nil {
			return state, err
		}
	} else {
		if err := fillJoinSetup(ctx, ui, &state); err != nil {
			return state, err
		}
	}

	if err := state.Validate(); err != nil {
		return state, err
	}
	return state, nil
}

func fillHostSetup(ui *lineUI, state *SetupState) error {
	room, err := ui.ask("Public room name")
	if err != nil {
		return err
	}
	state.Room = room

	protectedChoice, err := ui.askChoice("1. Open room\n2. Password-protected room\n", []string{"1", "2"})
	if err != nil {
		return err
	}
	state.Protected = protectedChoice == "2"

	if state.Protected {
		pass, err := ui.ask("Passcode")
		if err != nil {
			return err
		}
		state.Passcode = pass
	}
	return nil
}

func fillJoinSetup(ctx context.Context, ui *lineUI, state *SetupState) error {
	fmt.Fprintln(ui.out, "Searching LAN rooms...")
	records, err := discovery.Browse(ctx, 2*time.Second)
	if err != nil {
		fmt.Fprintf(ui.out, "Discovery failed: %v\n", err)
	}
	sortRecords(records)

	if len(records) == 0 {
		fmt.Fprintln(ui.out, "No LAN rooms found. Manual fallback.")
		addr, err := ui.ask("Server IP:port")
		if err != nil {
			return err
		}
		room, err := ui.ask("Room name")
		if err != nil {
			return err
		}
		protectedChoice, err := ui.askChoice("1. Open room\n2. Password-protected room\n", []string{"1", "2"})
		if err != nil {
			return err
		}
		state.ManualAddr = addr
		state.Room = room
		state.Protected = protectedChoice == "2"
	} else {
		fmt.Fprintln(ui.out)
		fmt.Fprintln(ui.out, "Rooms on this LAN:")
		for i, record := range records {
			kind := "open"
			if record.Protected {
				kind = "protected"
			}
			fmt.Fprintf(ui.out, "%d. %s [%s] %s:%d\n", i+1, record.Room, kind, record.Addr, record.Port)
		}
		fmt.Fprintln(ui.out, "m. Manual IP:port")

		choice, err := ui.ask("Select room")
		if err != nil {
			return err
		}

		if strings.EqualFold(choice, "m") {
			addr, err := ui.ask("Server IP:port")
			if err != nil {
				return err
			}
			room, err := ui.ask("Room name")
			if err != nil {
				return err
			}
			protectedChoice, err := ui.askChoice("1. Open room\n2. Password-protected room\n", []string{"1", "2"})
			if err != nil {
				return err
			}
			state.ManualAddr = addr
			state.Room = room
			state.Protected = protectedChoice == "2"
		} else {
			var idx int
			if _, err := fmt.Sscanf(choice, "%d", &idx); err != nil || idx < 1 || idx > len(records) {
				return fmt.Errorf("invalid room selection")
			}
			selected := records[idx-1]
			state.SelectedPeer = &selected
			state.Room = selected.Room
			state.Protected = selected.Protected
		}
	}

	if state.Protected {
		pass, err := ui.ask("Passcode")
		if err != nil {
			return err
		}
		state.Passcode = pass
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
