package tui

import (
	"context"
	"fmt"
	"io"
	"net"

	"termchat/internal/discovery"
	"termchat/internal/protocol"
)

func Run(ctx context.Context, in io.Reader, out io.Writer) error {
	ui := newLineUI(in, out)

	state, err := runSetup(ctx, ui)
	if err != nil {
		return err
	}

	addr := state.TargetAddr()
	localInfo := addr

	var advertiser *discovery.Advertiser
	if state.Mode == ModeHost {
		room := hostRoomConfig(state)
		_, port, err := protocol.StartHost(ctx, []protocol.RoomConfig{room})
		if err != nil {
			return err
		}

		advertiser, err = discovery.Advertise(room, port)
		if err != nil {
			return err
		}
		defer advertiser.Close()

		localIP := localIPv4()
		localInfo = net.JoinHostPort(localIP, fmt.Sprintf("%d", port))
		addr = net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))

		fmt.Fprintln(ui.out)
		fmt.Fprintf(ui.out, "Hosting room=%s\n", room.Name)
		fmt.Fprintf(ui.out, "local IP=%s\n", localIP)
		fmt.Fprintf(ui.out, "port=%d\n", port)
	}

	client, err := protocol.Dial(ctx, addr, state.Room, state.Nick, state.Passcode)
	if err != nil {
		return err
	}
	defer client.Close()

	return runChat(ctx, ui, client, state, localInfo)
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
