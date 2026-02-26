# satel

Go client library for the **Satel Integra** alarm system. It talks to an Integra control panel (or an ETHM Ethernet module) over TCP using the Satel frame protocol.

## Features

- Connect with address + 4-digit usercode, or wrap your own `net.Conn` (e.g. TLS)
- Query zones, partitions, and outputs (names and config)
- Subscribe to real-time state (zones, partitions, outputs, troubles)
- Arm/disarm partitions (normal, no-delay, force), bypass zones, set outputs
- Configurable timeouts and keepalive; handler can be nil for script-style use
- Safe for concurrent use; idempotent `Close()`

## Requirements

- Go 1.21+
- Integra panel with TCP access (direct or via ETHM-1/ETHM-2)

## Installation

```bash
go get github.com/eyetowers/satel
```

## Quick start

```go
package main

import (
	"log"
	"github.com/eyetowers/satel"
)

func main() {
	// Connect to the panel (e.g. ETHM at 192.168.1.100:7094).
	// Handler can be nil if you only send commands and don't care about events.
	client, err := satel.New("192.168.1.100:7094", "1234", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Optional: subscribe to zone/partition/output changes (needs a non-nil handler in production).
	_ = client.Subscribe(satel.ZoneViolation, satel.ArmedPartition)

	// Query or control.
	zones, err := client.GetZones()
	if err != nil {
		log.Fatal(err)
	}
	for _, z := range zones {
		log.Printf("Zone %d: %s (partition %s)", z.ID, z.Name, z.Partition.Name)
	}

	// Arm partition 1 in mode 0 (e.g. full arm).
	if err := client.ArmPartition(0, 1); err != nil {
		log.Fatal(err)
	}
}
```

## Connection

**Standard TCP:**

```go
client, err := satel.New("host:port", "0000", handler)
```

**Existing connection (e.g. TLS, test double):**

```go
conn, err := tls.Dial("tcp", addr, &tls.Config{...})
// ...
client, err := satel.NewWithConn(conn, "0000", satel.WithHandler(handler))
```

**Options** (for `NewWithConn`, or to override defaults):

- `satel.WithHandler(h)` – state and error callbacks (nil = no-op)
- `satel.WithCmdTimeout(d)` – timeout for request/response (default 10s)
- `satel.WithKeepAliveInterval(d)` – ping interval (default 20s)

## Handling events and errors

Implement the `satel.Handler` interface to receive zone/partition/output changes and connection errors. For a minimal implementation, embed `satel.IgnoreHandler` and override only the methods you need:

```go
type myHandler struct {
	satel.IgnoreHandler
}

func (myHandler) OnZoneViolations(index int, state, initial bool) {
	log.Printf("zone %d violation: %v (initial=%v)", index, state, initial)
}

func (myHandler) OnError(err error) {
	log.Printf("satel error: %v", err)
}

client, err := satel.New(addr, "0000", myHandler{})
```

## Documentation

- API: [pkg.go.dev/github.com/eyetowers/satel](https://pkg.go.dev/github.com/eyetowers/satel)
- The library follows the Satel Integra TCP/ETHM protocol (frame format, commands, and state subscription). Protocol details are described in Satel’s technical documentation for Integra and ETHM modules.
