package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"termchat/internal/protocol"
)

func runChat(ctx context.Context, ui *lineUI, client *protocol.Client, state SetupState, localInfo string) error {
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, "Chat view")
	fmt.Fprintf(ui.out, "room=%s local=%s\n", state.Room, localInfo)
	fmt.Fprintln(ui.out, "Type messages. /quit to exit.")
	fmt.Fprintln(ui.out)

	events := client.Receive(ctx)
	inputDone := make(chan error, 1)

	go func() {
		for {
			line, err := ui.ask(">")
			if err != nil {
				inputDone <- err
				return
			}
			if line == "" {
				continue
			}
			if line == "/quit" {
				inputDone <- nil
				return
			}
			if err := client.Send(line); err != nil {
				inputDone <- err
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-inputDone:
			return err
		case event, ok := <-events:
			if !ok {
				return nil
			}
			renderEvent(ui.out, event)
		}
	}
}

func renderEvent(out io.Writer, event protocol.Event) {
	switch event.Kind {
	case protocol.EventMessage:
		fmt.Fprintf(out, "[%s] %s: %s\n", event.SentAt.Local().Format(time.Kitchen), event.Sender, event.Body)
	case protocol.EventParticipantJoined:
		fmt.Fprintf(out, "* %s joined\n", event.Sender)
	case protocol.EventParticipantLeft:
		fmt.Fprintf(out, "* %s left\n", event.Sender)
	case protocol.EventError:
		fmt.Fprintf(out, "! %s\n", strings.Trim(event.Body, `"`))
	}
}
