package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type Client struct {
	conn     net.Conn
	room     string
	nick     string
	passcode string
	reader   *bufio.Reader
}

type EventKind string

const (
	EventMessage           EventKind = "message"
	EventParticipantJoined EventKind = "joined"
	EventParticipantLeft   EventKind = "left"
	EventError             EventKind = "error"
)

type Event struct {
	Kind   EventKind
	Sender string
	Body   string
	SentAt time.Time
}

func Dial(ctx context.Context, addr, room, nick, passcode string) (*Client, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	hello, err := json.Marshal(HelloPayload{
		Nickname: nick,
		Verifier: AuthVerifier(room, passcode),
	})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if err := WriteFrame(conn, ControlFrame{
		Type:    FrameTypeHello,
		Room:    room,
		Sender:  nick,
		Payload: hello,
	}); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &Client{
		conn:     conn,
		room:     room,
		nick:     nick,
		passcode: passcode,
		reader:   bufio.NewReader(conn),
	}, nil
}

func (c *Client) Send(body string) error {
	env, err := EncryptEnvelope(c.room, c.nick, c.passcode, body, time.Now())
	if err != nil {
		return err
	}

	payload, err := json.Marshal(env)
	if err != nil {
		return err
	}

	return WriteFrame(c.conn, ControlFrame{
		Type:    FrameTypeMsg,
		Room:    c.room,
		Sender:  c.nick,
		Payload: payload,
	})
}

func (c *Client) Receive(ctx context.Context) <-chan Event {
	out := make(chan Event, 16)

	go func() {
		defer close(out)

		go func() {
			<-ctx.Done()
			_ = c.conn.SetReadDeadline(time.Now())
		}()

		for {
			frame, err := ReadFrame(c.reader)
			if err != nil {
				return
			}

			switch frame.Type {
			case FrameTypeJoined:
				out <- Event{Kind: EventParticipantJoined, Sender: frame.Sender}
			case FrameTypeLeft:
				out <- Event{Kind: EventParticipantLeft, Sender: frame.Sender}
			case FrameTypeError:
				out <- Event{Kind: EventError, Sender: "system", Body: string(frame.Payload)}
			case FrameTypeMsg:
				var env Envelope
				if err := json.Unmarshal(frame.Payload, &env); err != nil {
					out <- Event{Kind: EventError, Sender: "system", Body: "invalid message frame"}
					continue
				}
				msg, err := DecryptEnvelope(env, c.passcode)
				if err != nil {
					out <- Event{Kind: EventError, Sender: env.Sender, Body: "failed to decrypt message"}
					continue
				}
				out <- Event{Kind: EventMessage, Sender: env.Sender, Body: msg.Body, SentAt: env.SentAt}
			}
		}
	}()

	return out
}

func (c *Client) Close() error {
	return c.conn.Close()
}
