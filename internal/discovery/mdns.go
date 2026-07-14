package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"

	"termchat/internal/protocol"
)

type DiscoveryRecord struct {
	Room            string
	Hostname        string
	Addr            string
	Port            int
	ProtocolVersion string
	Protected       bool
}

type Advertiser struct {
	server *zeroconf.Server
}

func Advertise(cfg protocol.RoomConfig, port int) (*Advertiser, error) {
	txt := []string{
		"room=" + cfg.Name,
		"protocol=" + protocol.Version,
		"protected=" + boolString(cfg.Protected),
	}
	server, err := zeroconf.Register(cfg.Name, protocol.ServiceName, "local.", port, txt, nil)
	if err != nil {
		return nil, fmt.Errorf("mdns register: %w", err)
	}
	return &Advertiser{server: server}, nil
}

func (a *Advertiser) Close() {
	if a != nil && a.server != nil {
		a.server.Shutdown()
	}
}

func Browse(ctx context.Context, timeout time.Duration) ([]DiscoveryRecord, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("mdns resolver: %w", err)
	}

	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)
	out := make(chan DiscoveryRecord, 16)

	go func() {
		defer close(out)
		for entry := range entries {
			record := DiscoveryRecord{
				Hostname: entry.HostName,
				Port:     entry.Port,
			}
			if len(entry.AddrIPv4) > 0 {
				record.Addr = entry.AddrIPv4[0].String()
			} else if len(entry.AddrIPv6) > 0 {
				record.Addr = entry.AddrIPv6[0].String()
			}
			for _, txt := range entry.Text {
				key, value, ok := strings.Cut(txt, "=")
				if !ok {
					continue
				}
				switch key {
				case "room":
					record.Room = value
				case "protocol":
					record.ProtocolVersion = value
				case "protected":
					record.Protected = value == "true"
				}
			}
			if record.Room != "" && record.Addr != "" {
				out <- record
			}
		}
	}()

	if err := resolver.Browse(browseCtx, protocol.ServiceName, "local.", entries); err != nil {
		return nil, fmt.Errorf("mdns browse: %w", err)
	}
	<-browseCtx.Done()

	seen := map[string]struct{}{}
	var records []DiscoveryRecord
	for record := range out {
		key := fmt.Sprintf("%s|%s|%d", record.Room, record.Addr, record.Port)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		records = append(records, record)
	}

	return records, nil
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
