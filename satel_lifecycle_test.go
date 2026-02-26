package satel

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// silentClose closes c and ignores the error. Use in defers to satisfy errcheck.
func silentClose(c io.Closer) { _ = c.Close() }

// mockSatelServer starts a TCP server that speaks the Satel frame protocol and
// responds to device info (0x7E), keepalive (0x7C), and status (0xEF) so the
// client can complete New() and one round-trip. It accepts one connection and
// runs the handler in a goroutine. Call Close() on the returned listener to
// stop the server.
func mockSatelServer(t *testing.T) (listenAddr string, listener net.Listener) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	// Device info response: INTEGRA24 (0x00), version "1.00 2020-01", language Polish (0). Must be 14 bytes.
	deviceInfoPayload := []byte{
		0x00, 0x31, 0x2E, 0x30, 0x30, 0x20, 0x32, 0x30,
		0x32, 0x30, 0x2D, 0x30, 0x31, 0x00,
	}
	if len(deviceInfoPayload) != 14 {
		t.Fatalf("device info payload must be 14 bytes, got %d", len(deviceInfoPayload))
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer silentClose(conn)

		scanner := bufio.NewScanner(conn)
		scanner.Split(scan)
		for scanner.Scan() {
			payload := scanner.Bytes()
			cmd, _, err := decomposePayload(payload...)
			if err != nil {
				return
			}
			switch cmd {
			case SatelDeviceInfoCmd:
				_, _ = conn.Write(frame(append([]byte{SatelDeviceInfoCmd}, deviceInfoPayload...)...))
			case SatelDeviceVersion, ResponseStatusCmd:
				_, _ = conn.Write(frame(ResponseStatusCmd, byte(Ok)))
			default:
				_, _ = conn.Write(frame(ResponseStatusCmd, byte(Ok)))
			}
		}
	}()

	return listener.Addr().String(), listener
}

func TestNew_WithMockServer(t *testing.T) {
	// Given.
	addr, listener := mockSatelServer(t)
	defer silentClose(listener)

	// When.
	s, err := New(addr, "0000", IgnoreHandler{})
	require.NoError(t, err)
	closeErr := s.Close()

	// Then.
	require.NotNil(t, s)
	require.NoError(t, closeErr)
}

// TestClose_DoubleClose_BothReturnNil locks in the contract: calling Close() twice
// is safe and both calls return nil (the first performs the close, the second is a no-op).
func TestClose_DoubleClose_BothReturnNil(t *testing.T) {
	// Given.
	addr, listener := mockSatelServer(t)
	defer silentClose(listener)
	s, err := New(addr, "0000", IgnoreHandler{})
	require.NoError(t, err)

	// When.
	firstErr := s.Close()
	secondErr := s.Close()

	// Then.
	require.NoError(t, firstErr, "first Close() must return nil")
	require.NoError(t, secondErr, "second Close() must return nil (idempotent contract)")
}

func TestNew_WithMockServer_NilHandler(t *testing.T) {
	// Given.
	addr, listener := mockSatelServer(t)
	defer silentClose(listener)

	// When.
	s, err := New(addr, "0000", nil)
	require.NoError(t, err)
	closeErr := s.Close()

	// Then.
	require.NotNil(t, s)
	require.NoError(t, closeErr)
}

func TestNew_WithMockServer_SubscribeRoundTrip(t *testing.T) {
	// Given.
	addr, listener := mockSatelServer(t)
	defer silentClose(listener)
	s, err := New(addr, "0000", IgnoreHandler{})
	require.NoError(t, err)
	defer silentClose(s)

	// When.
	err = s.Subscribe(ZoneViolation)

	// Then.
	require.NoError(t, err)
}

func TestNewWithConn_WithMockServer(t *testing.T) {
	// Given.
	addr, listener := mockSatelServer(t)
	defer silentClose(listener)
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer silentClose(conn)

	// When.
	s, err := NewWithConn(conn, "0000")
	require.NoError(t, err)
	defer silentClose(s)
	err = s.Subscribe(ZoneViolation)

	// Then.
	require.NoError(t, err)
}

func TestClose_AfterClose_ReturnsErrorOnSend(t *testing.T) {
	// Given.
	addr, listener := mockSatelServer(t)
	defer silentClose(listener)
	s, err := New(addr, "0000", IgnoreHandler{})
	require.NoError(t, err)
	err = s.Close()
	require.NoError(t, err)

	// When.
	err = s.Subscribe(ZoneViolation)

	// Then.
	require.Error(t, err)
}

func TestConcurrentCloseAndSendCmd(t *testing.T) {
	// Given.
	addr, listener := mockSatelServer(t)
	defer silentClose(listener)
	s, err := New(addr, "0000", IgnoreHandler{})
	require.NoError(t, err)

	var sendErr error
	var wg sync.WaitGroup

	// When.
	wg.Go(func() {
		sendErr = s.Subscribe(ZoneViolation)
	})
	closeErr := s.Close()
	wg.Wait()

	// Then.
	require.NoError(t, closeErr)
	require.Error(t, sendErr)
	require.True(t, errIsOrWraps(sendErr, ErrDisconnected) || isClosedConnError(sendErr),
		"sendErr should be ErrDisconnected or closed connection error, got: %v", sendErr)
}

func TestReturnResponse_ReceiverStartsLate_NoDrop(t *testing.T) {
	// Given.
	waiterCh := make(chan Response, 1)
	s := &Satel{
		handler: IgnoreHandler{},
	}
	s.setResponseWaiter(&responseWaiter{
		expectedCmd: SatelDeviceVersion,
		ch:          waiterCh,
	})

	done := make(chan struct{})

	// When.
	go func() {
		s.returnResponse(ResponseStatusCmd, byte(Ok))
		close(done)
	}()
	<-done // ensure returnResponse ran before receiver starts reading

	// Then.
	select {
	case resp := <-waiterCh:
		require.Equal(t, ResponseStatusCmd, resp.cmd)
		require.Equal(t, Ok, resp.status)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected response to be available for late receiver")
	}
}

func TestSendCmd_DiscardsStaleBufferedResponse(t *testing.T) {
	// Given.
	wrote := make(chan struct{})
	var wroteOnce sync.Once
	s := &Satel{
		conn: stubConn{
			onWrite: func([]byte) {
				wroteOnce.Do(func() { close(wrote) })
			},
		},
		handler:    IgnoreHandler{},
		cmdTimeout: 100 * time.Millisecond,
	}
	// Simulate a stale response when no waiter is active.
	s.returnResponse(ResponseStatusCmd, byte(OtherError))

	// Send the current response only after sendCmd has actually written the command.
	go func() {
		<-wrote
		s.returnResponse(ResponseStatusCmd, byte(Ok))
	}()

	// When.
	resp, err := s.sendCmd(SatelDeviceVersion)

	// Then.
	require.NoError(t, err)
	require.Equal(t, ResponseStatusCmd, resp.cmd)
	require.Equal(t, Ok, resp.status)
}

func TestSendCmd_SequentialImmediateResponsesSucceed(t *testing.T) {
	// Given.
	statuses := []ResponseStatus{Ok, CommandAccepted, Ok}
	var idx int
	var s *Satel
	c := stubConn{
		onWrite: func([]byte) {
			// onWrite runs synchronously in stubConn.Write, so the response is
			// queued before sendCmd starts waiting on waiter.ch.
			if idx >= len(statuses) {
				return
			}
			status := statuses[idx]
			idx++
			s.returnResponse(ResponseStatusCmd, byte(status))
		},
	}
	s = &Satel{
		conn:       c,
		handler:    IgnoreHandler{},
		cmdTimeout: 100 * time.Millisecond,
	}

	sendAndExpect := func() {
		// When.
		resp, err := s.sendCmd(SatelDeviceVersion)

		// Then.
		require.NoError(t, err)
		require.Equal(t, ResponseStatusCmd, resp.cmd)
		require.Equal(t, statuses[idx-1], resp.status)
	}

	// When.
	sendAndExpect()
	sendAndExpect()
	sendAndExpect()
}

func TestSendCmd_ChannelClosed_ReturnsDisconnected(t *testing.T) {
	// Given.
	done := make(chan bool)
	close(done)
	s := &Satel{
		conn:       stubConn{},
		cmdTimeout: 100 * time.Millisecond,
		done:       done,
	}

	// When.
	_, err := s.sendCmd(SatelDeviceVersion)

	// Then.
	require.ErrorIs(t, err, ErrDisconnected)
}

func TestRequiresReconnect_TrueAfterTimeout(t *testing.T) {
	// Given.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer silentClose(listener)

	deviceInfoPayload := []byte{
		0x00, 0x31, 0x2E, 0x30, 0x30, 0x20, 0x32, 0x30,
		0x32, 0x30, 0x2D, 0x30, 0x31, 0x00,
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer silentClose(conn)

		scanner := bufio.NewScanner(conn)
		scanner.Split(scan)

		droppedOnce := false
		for scanner.Scan() {
			payload := scanner.Bytes()
			cmd, _, err := decomposePayload(payload...)
			if err != nil {
				return
			}
			switch cmd {
			case SatelDeviceInfoCmd:
				_, _ = conn.Write(frame(append([]byte{SatelDeviceInfoCmd}, deviceInfoPayload...)...))
			default:
				// Drop first subscribe request to force timeout.
				if cmd == subscribeCmd && !droppedOnce {
					droppedOnce = true
					continue
				}
				_, _ = conn.Write(frame(ResponseStatusCmd, byte(Ok)))
			}
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer silentClose(conn)

	s, err := NewWithConn(
		conn,
		"0000",
		WithHandler(IgnoreHandler{}),
		WithCmdTimeout(40*time.Millisecond),
		WithKeepAliveInterval(time.Hour),
	)
	require.NoError(t, err)
	defer silentClose(s)

	// When.
	err = s.Subscribe(ZoneViolation)

	// Then.
	require.ErrorIs(t, err, ErrTimeout)
	require.True(t, s.RequiresReconnect())
}

func TestRequiresReconnect_FalseOnNonTerminalStatusError(t *testing.T) {
	// Given.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer silentClose(listener)

	deviceInfoPayload := []byte{
		0x00, 0x31, 0x2E, 0x30, 0x30, 0x20, 0x32, 0x30,
		0x32, 0x30, 0x2D, 0x30, 0x31, 0x00,
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer silentClose(conn)

		scanner := bufio.NewScanner(conn)
		scanner.Split(scan)
		for scanner.Scan() {
			payload := scanner.Bytes()
			cmd, _, err := decomposePayload(payload...)
			if err != nil {
				return
			}
			switch cmd {
			case SatelDeviceInfoCmd:
				_, _ = conn.Write(frame(append([]byte{SatelDeviceInfoCmd}, deviceInfoPayload...)...))
			case subscribeCmd:
				_, _ = conn.Write(frame(ResponseStatusCmd, byte(NoAccess)))
			default:
				_, _ = conn.Write(frame(ResponseStatusCmd, byte(Ok)))
			}
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer silentClose(conn)

	s, err := NewWithConn(
		conn,
		"0000",
		WithHandler(IgnoreHandler{}),
		WithCmdTimeout(200*time.Millisecond),
		WithKeepAliveInterval(time.Hour),
	)
	require.NoError(t, err)
	defer silentClose(s)

	// When.
	err = s.Subscribe(ZoneViolation)

	// Then.
	require.Error(t, err)
	require.False(t, s.RequiresReconnect())
}

func errIsOrWraps(err, target error) bool {
	return err != nil && errors.Is(err, target)
}

func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "connection reset")
}

type stubConn struct {
	onWrite func([]byte)
}

func (stubConn) Read(_ []byte) (int, error) { return 0, io.EOF }
func (c stubConn) Write(b []byte) (int, error) {
	if c.onWrite != nil {
		c.onWrite(b)
	}
	return len(b), nil
}
func (stubConn) Close() error                       { return nil }
func (stubConn) LocalAddr() net.Addr                { return stubAddr("local") }
func (stubConn) RemoteAddr() net.Addr               { return stubAddr("remote") }
func (stubConn) SetDeadline(_ time.Time) error      { return nil }
func (stubConn) SetReadDeadline(_ time.Time) error  { return nil }
func (stubConn) SetWriteDeadline(_ time.Time) error { return nil }

type stubAddr string

func (a stubAddr) Network() string { return "stub" }
func (a stubAddr) String() string  { return string(a) }
