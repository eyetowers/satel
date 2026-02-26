// Package satel provides a Go client for the Satel Integra alarm system.
// It communicates over TCP with the Integra panel (or an ETHM module) using
// the Satel frame protocol. Use New to connect, then call methods to query
// zones/outputs, subscribe to state updates, arm/disarm partitions, and
// control outputs. The Handler interface receives real-time state changes
// and errors. Call Close when done to release the connection.
package satel

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by the client or by validation.
var (
	ErrDisconnected      = errors.New("disconnected")
	ErrCrcNotMatch       = errors.New("corrupt response: crc does not match")
	ErrCorruptedResponse = errors.New("corrupted response: does not match the documentation")
	ErrForbiddenCommand  = errors.New("forbidden command value")
	ErrNoConnection      = errors.New("no connection")
	ErrReturnResponse    = errors.New("failed returning response, unexpectedly no caller available")
	ErrProtocolViolation = errors.New("response violates protocol")
	ErrDeviceNotFound    = errors.New("requested device not found")
	ErrTimeout           = fmt.Errorf("timeout (%s), no response", CmdTimeout.String())
)

// ShouldReconnect reports whether err indicates the client session is no longer
// safe to use and the caller should create a new Satel instance.
// It also returns true for any net.Error (including temporary transport errors).
// Callers can retry the same operation on a newly created client if needed.
func ShouldReconnect(err error) bool {
	if err == nil {
		return false
	}
	terminal := []error{
		ErrTimeout,
		ErrDisconnected,
		ErrNoConnection,
		ErrCrcNotMatch,
		ErrCorruptedResponse,
		ErrProtocolViolation,
	}
	for _, target := range terminal {
		if errors.Is(err, target) {
			return true
		}
	}
	// Transport-level failures are terminal for this session.
	var netErr net.Error
	return errors.As(err, &netErr)
}

// RequiresReconnect reports whether this client instance is no longer safe to
// use and the caller should create a new Satel instance.
// Once true, it stays true for the lifetime of the client.
func (s *Satel) RequiresReconnect() bool {
	return s != nil && s.invalid.Load()
}

// KeepAliveInterval is how often the client pings the device to keep the TCP connection alive.
// CmdTimeout is how long Send-style operations wait for a response before returning ErrTimeout.
const (
	KeepAliveInterval = 20 * time.Second
	CmdTimeout        = 10 * time.Second

	ResponseStatusCmd  = byte(0xEF)
	SatelDeviceInfoCmd = byte(0x7E)
	SatelDeviceVersion = byte(0x7C)
	ReadDeviceCmd      = byte(0xEE)
)

// Satel is a client for a Satel Integra alarm panel (or ETHM). It is safe for concurrent use.
// Create one with New or NewWithConn and call Close when done.
type Satel struct {
	conn               net.Conn
	usercode           []byte
	mu                 sync.Mutex
	waiterMu           sync.Mutex
	cmdSize            int
	zoneOutputCapacity uint64
	deviceLanguage     language

	keepAliveInterval time.Duration
	cmdTimeout        time.Duration

	responseWaiter *responseWaiter
	handler        Handler
	closing        atomic.Bool
	invalid        atomic.Bool
	done           chan bool

	closeOnce sync.Once
}

// Option configures a Satel client. Used with New and NewWithConn.
type Option func(*clientOptions)

type clientOptions struct {
	handler           Handler
	keepAliveInterval time.Duration
	cmdTimeout        time.Duration
}

// WithHandler sets the handler for state updates and errors. If not set, or set to nil,
// a no-op handler is used. For production use a real handler to react to events and errors.
func WithHandler(h Handler) Option {
	return func(o *clientOptions) {
		o.handler = h
	}
}

// WithKeepAliveInterval sets how often the client pings the device to keep the TCP connection alive.
// If not set, KeepAliveInterval (20s) is used.
func WithKeepAliveInterval(d time.Duration) Option {
	return func(o *clientOptions) {
		o.keepAliveInterval = d
	}
}

// WithCmdTimeout sets how long request/response operations wait for a reply before returning ErrTimeout.
// If not set, CmdTimeout (10s) is used.
func WithCmdTimeout(d time.Duration) Option {
	return func(o *clientOptions) {
		o.cmdTimeout = d
	}
}

func applyOptions(opts []Option) clientOptions {
	o := clientOptions{
		keepAliveInterval: KeepAliveInterval,
		cmdTimeout:        CmdTimeout,
	}
	for _, f := range opts {
		f(&o)
	}
	if o.handler == nil {
		o.handler = IgnoreHandler{}
	}
	if o.keepAliveInterval <= 0 {
		o.keepAliveInterval = KeepAliveInterval
	}
	if o.cmdTimeout <= 0 {
		o.cmdTimeout = CmdTimeout
	}
	return o
}

type Response struct {
	cmd    byte
	data   []byte
	status ResponseStatus
}

type responseWaiter struct {
	expectedCmd byte
	ch          chan Response
}

// Zone represents a single zone (sensor/detector) on the panel.
type Zone struct {
	ID        uint64
	Name      string
	Partition Partition
}

// Partition represents an armed area; zones can be assigned to partitions.
type Partition struct {
	ID   uint64
	Name string
}

// Output represents a relay/output on the panel.
type Output struct {
	ID       uint64
	Name     string
	Function OutputFunction
}

// New connects to the Integra at address (e.g. "host:port"), validates usercode (4 digits),
// and performs device handshake. The handler h receives state updates and errors; it may be nil
// to use a no-op handler (e.g. for scripts that only send commands). Call Close when done.
func New(address, usercode string, h Handler) (*Satel, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connection to %s failed with error: %w", address, err)
	}

	err = validateUsercode(usercode)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("validating usercode: %w", err)
	}

	return newConfig(conn, usercode, WithHandler(h))
}

// NewWithConn builds a client from an existing connection (e.g. TLS wrapper or test double).
// Usercode is validated. Closing the returned client will close conn. Options can set handler,
// keepalive interval, and command timeout.
func NewWithConn(conn net.Conn, usercode string, opts ...Option) (*Satel, error) {
	if err := validateUsercode(usercode); err != nil {
		return nil, fmt.Errorf("validating usercode: %w", err)
	}
	return newConfig(conn, usercode, opts...)
}

func newConfig(conn net.Conn, usercode string, opts ...Option) (*Satel, error) {
	o := applyOptions(opts)
	s := &Satel{
		conn:               conn,
		usercode:           transformCode(usercode),
		keepAliveInterval:  o.keepAliveInterval,
		cmdTimeout:         o.cmdTimeout,
		handler:            o.handler,
		cmdSize:            16,
		zoneOutputCapacity: 0,
		done:               make(chan bool),
	}

	go s.read()

	model, version, language, err := s.getSatelDeviceInfo()
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("getting satel device info : %w", err)
	}
	if version[0] == '2' && model == INTEGRA256Plus {
		_ = s.Close()
		return nil, fmt.Errorf("satel device model %q not yet supported", model.String())
	}

	s.zoneOutputCapacity, err = model.ZoneAndOutputCapacity()
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("getting zone and output capacity : %w", err)
	}

	s.deviceLanguage = language
	go s.keepConnectionAlive()

	return s, nil
}

func (s *Satel) keepConnectionAlive() {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-timer.C:
		}

		// Sending this random command just to keep the connection alive.
		_, err := s.sendCmd(SatelDeviceVersion)
		if err != nil {
			if s.closing.Load() || errors.Is(err, ErrDisconnected) {
				return
			}
			s.reportError(fmt.Errorf("keeping connection alive: %w", err))
			return
		}
		timer.Reset(s.keepAliveInterval)
	}
}

func (s *Satel) getDeviceName(deviceType byte, deviceID int, expectedResposeSize int) (*Response, error) {
	resp, err := s.sendCmd(ReadDeviceCmd, deviceType, byte(deviceID))
	if err != nil {
		return nil, fmt.Errorf("getting device (ID: %d) name : %w", deviceID, err)
	}

	if resp.cmd != ReadDeviceCmd && resp.cmd != ResponseStatusCmd {
		return nil, fmt.Errorf("getting device(%d) information, response does not match the command: %w",
			deviceID, ErrProtocolViolation,
		)
	}

	if resp.cmd == ResponseStatusCmd {
		// When error is "Other Error" that means requested device is not found in Satel.
		if resp.status == OtherError {
			return nil, ErrDeviceNotFound
		}
		return nil, fmt.Errorf("unexpected response status while getting device name(%d) : %w",
			deviceID, ErrProtocolViolation)
	}

	if len(resp.data) != expectedResposeSize {
		return nil, fmt.Errorf("unexpected payload size, expected %dB, actual %dB: %w",
			expectedResposeSize, len(resp.data), ErrProtocolViolation)
	}
	return resp, nil
}

func (s *Satel) GetOutputs() ([]Output, error) {
	supportedOutputs := int(s.zoneOutputCapacity)
	outputDevice := byte(0x04)
	expectedResposeSize := 19
	var outputs []Output

	for i := 1; i < supportedOutputs; i++ {
		resp, err := s.getDeviceName(outputDevice, i, expectedResposeSize)
		if err != nil && err != ErrDeviceNotFound {
			return nil, fmt.Errorf("getting output device(%d) name: %w", i, err)
		}
		if err == ErrDeviceNotFound {
			continue
		}

		deviceType, outputID, outputFunc, outputName := decodeOutput(resp.data, s.deviceLanguage)
		if outputDevice != deviceType {
			return nil, fmt.Errorf("getting output(%d) information, received response is not for output: %w", outputID, ErrProtocolViolation)
		}

		output := Output{
			ID:       outputID,
			Name:     outputName,
			Function: outputFunc,
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

// GetZones returns all configured zones (sensors) with names and partition assignment.
func (s *Satel) GetZones() ([]Zone, error) {
	supportedZones := int(s.zoneOutputCapacity)
	zoneDevice := byte(0x05)
	expectedResposeSize := 20
	var zones []Zone

	partitions := make(map[uint64]Partition)
	for i := 1; i < supportedZones; i++ {
		resp, err := s.getDeviceName(zoneDevice, i, expectedResposeSize)
		if err != nil && err != ErrDeviceNotFound {
			return nil, fmt.Errorf("getting zone(%d) name: %w", i, err)
		}
		if err == ErrDeviceNotFound {
			continue
		}

		deviceType, zoneID, name, partitionID := decodeZone(resp.data, s.deviceLanguage)
		if zoneDevice != deviceType {
			return nil, fmt.Errorf("getting zone(%d) information, received response is not for zone: %w", i, ErrProtocolViolation)
		}

		partition, exists := partitions[partitionID]
		if !exists {
			partition, err = s.getPartition(partitionID)
			if err != nil {
				return nil, err
			}
			partitions[partitionID] = partition
		}
		zone := Zone{
			ID:        zoneID,
			Name:      name,
			Partition: partition,
		}
		zones = append(zones, zone)
	}
	return zones, nil
}

func (s *Satel) getPartition(partition uint64) (Partition, error) {
	partitionDevice := byte(0x00)
	expectedResposeSize := 19

	resp, err := s.getDeviceName(partitionDevice, int(partition), expectedResposeSize)
	if err != nil {
		return Partition{}, fmt.Errorf("getting partition(%d) name: %w", partition, err)
	}

	deviceType, partitionID, partitionName := decodePartition(resp.data, s.deviceLanguage)
	if partitionDevice != deviceType {
		return Partition{}, fmt.Errorf("getting partition(%d) information, received response is not for partition: %w", partition, ErrProtocolViolation)
	}

	return Partition{
		ID:   partitionID,
		Name: partitionName,
	}, nil
}

// Subscribe enables push updates for the given state types. The panel will send frames
// whenever those states change; the client delivers them to the Handler.
func (s *Satel) Subscribe(states ...StateType) error {
	err := s.sendCmdWithResultCheck(transformSubscription(states...))
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	return nil
}

// ArmPartition arms the given partition in the specified mode (0–3).
func (s *Satel) ArmPartition(mode, partition int) error {
	bytes := s.prepareCommand(byte(0x80+mode), 4, partition)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) ArmPartitionNoDelay(mode, partition int) error {
	bytes := s.prepareCommand(byte(0x80+mode), 4, partition)
	bytes = append(bytes, 0x80)
	return s.sendCmdWithResultCheck(bytes)
}

// ForceArmPartition arms the partition even if zones are violated (e.g. open door).
func (s *Satel) ForceArmPartition(mode, partition int) error {
	bytes := s.prepareCommand(byte(0xA0+mode), 4, partition)
	return s.sendCmdWithResultCheck(bytes)
}

// ForceArmPartitionNoDelay force-arms the partition without exit delay.
func (s *Satel) ForceArmPartitionNoDelay(mode, partition int) error {
	bytes := s.prepareCommand(byte(0xA0+mode), 4, partition)
	bytes = append(bytes, 0x80)
	return s.sendCmdWithResultCheck(bytes)
}

// DisarmPartition disarms the given partition.
func (s *Satel) DisarmPartition(partition int) error {
	bytes := s.prepareCommand(byte(0x84), 4, partition)
	return s.sendCmdWithResultCheck(bytes)
}

// ClearAlarm clears the alarm for the given partition index.
func (s *Satel) ClearAlarm(index int) error {
	bytes := s.prepareCommand(byte(0x85), 4, index)
	return s.sendCmdWithResultCheck(bytes)
}

// AlarmCheck triggers alarm verification for the given partition index.
func (s *Satel) AlarmCheck(index int) error {
	bytes := s.prepareCommand(byte(0x13), 4, index)
	return s.sendCmdWithResultCheck(bytes)
}

// ZoneBypass bypasses the given zone (excludes it from arming).
func (s *Satel) ZoneBypass(zone int) error {
	bytes := s.prepareCommand(byte(0x86), s.cmdSize, zone)
	return s.sendCmdWithResultCheck(bytes)
}

// ZoneUnBypass removes bypass from the given zone.
func (s *Satel) ZoneUnBypass(zone int) error {
	bytes := s.prepareCommand(byte(0x87), s.cmdSize, zone)
	return s.sendCmdWithResultCheck(bytes)
}

// SetOutput sets the output at index to on (true) or off (false).
func (s *Satel) SetOutput(index int, value bool) error {
	cmd := byte(0x89)
	if value {
		cmd = 0x88
	}
	bytes := s.prepareCommand(cmd, s.cmdSize, index)
	return s.sendCmdWithResultCheck(bytes)
}

// ClearTroubleMemory clears the panel’s trouble memory (requires usercode).
func (s *Satel) ClearTroubleMemory() error {
	bytes := append([]byte{0x8B}, s.usercode...)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) prepareCommand(cmd byte, cmdSize int, index int) []byte {
	// Subtracting 1 from index since Satel indexes from 0.
	index = index - 1
	data := make([]byte, cmdSize)
	data[index/8] = 1 << (index % 8)
	bytes := append([]byte{cmd}, s.usercode...)
	return append(bytes, data...)
}

// Close shuts down the connection and blocks until the read loop has finished.
// It is safe to call multiple times; only the first call performs the close,
// and subsequent calls block until shutdown is complete then return nil.
func (s *Satel) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.closing.Store(true)
		err = s.conn.Close()
		<-s.done
	})
	if err != nil {
		return fmt.Errorf("closing satel connection: %w", err)
	}
	return nil
}

func (s *Satel) closeRead() {
	s.invalid.Store(true)
	_ = s.conn.Close()
	close(s.done)
}

func (s *Satel) reportError(err error) {
	if !s.closing.Load() && s.handler != nil {
		s.handler.OnError(err)
	}
}

type command struct {
	prev        [64]byte
	initialized bool
}

func (s *Satel) read() {
	scanner := bufio.NewScanner(s.conn)
	scanner.Split(scan)
	commands := make(map[byte]command)
	defer s.closeRead()
	for {
		ok := scanner.Scan()
		if !ok {
			if scanner.Err() == nil {
				s.reportError(ErrDisconnected)
			} else {
				s.reportError(scanner.Err())
			}
			return
		}

		bytes := scanner.Bytes()
		cmd, data, err := decomposePayload(bytes...)
		if err != nil {
			s.reportError(err)
			break
		}

		if cmd == ResponseStatusCmd || cmd == SatelDeviceInfoCmd || cmd == SatelDeviceVersion || cmd == ReadDeviceCmd {
			s.returnResponse(cmd, data...)
			continue
		}

		if StateType(cmd) > ZoneIsolate {
			s.reportError(fmt.Errorf("unknown state type from device: 0x%02X", cmd))
			continue
		}

		c := commands[cmd]
		// Process only up to len(c.prev) to avoid index out of range; protocol state payloads are expected to fit.
		dataLen := len(data)
		if dataLen > len(c.prev) {
			dataLen = len(c.prev)
		}
		for i := 0; i < dataLen; i++ {
			bb := data[i]
			change := bb ^ c.prev[i]
			for j := 0; j < 8; j++ {
				index := byte(1 << j)
				if !c.initialized || change&index != 0 {
					if cmd == byte(TroublePart3) {
						s.handleTroublePart3(i, j, bb, index, c)
						continue
					}

					handleChange := handlerFunc(s.handler, StateType(cmd))
					if !s.closing.Load() {
						// Adding 1 to index since Satel indexes from 0.
						handleChange(((i * 8) + j + 1), bb&index != 0, !c.initialized)
					}
				}
			}
			c.prev[i] = data[i]
		}
		c.initialized = true
		commands[cmd] = c
	}
}

func (s *Satel) handleTroublePart3(i, j int, bb, index byte, c command) {
	byteSegment := 15
	troubleType := i / byteSegment
	idx := (i % byteSegment * 8) + j + 1
	if !s.closing.Load() {
		s.handler.OnTroublePart3(idx, Trouble3Type(troubleType), bb&index != 0, !c.initialized)
	}
}

func (s *Satel) returnResponse(cmd byte, data ...byte) {
	resp := Response{
		cmd: cmd,
	}

	if cmd == ResponseStatusCmd {
		resp.status = ResponseStatus(data[0])
	} else {
		// Copy so the scanner's buffer is not shared with other goroutines.
		resp.data = bytes.Clone(data)
	}

	waiter := s.getResponseWaiter()
	if waiter == nil {
		s.reportError(ErrReturnResponse)
		return
	}
	if !isExpectedResponse(waiter.expectedCmd, resp.cmd) {
		s.reportError(fmt.Errorf("ignoring mismatched response: got 0x%02X for request 0x%02X", resp.cmd, waiter.expectedCmd))
		return
	}

	select {
	case waiter.ch <- resp:
	default:
		s.reportError(ErrReturnResponse)
	}
}

func (s *Satel) sendCmd(data ...byte) (*Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.invalid.Load() {
		return nil, ErrDisconnected
	}

	select {
	case <-s.done:
		s.invalid.Store(true)
		return nil, ErrDisconnected
	default:
	}

	if s.conn == nil {
		return nil, ErrNoConnection
	}

	waiter := &responseWaiter{
		expectedCmd: data[0],
		ch:          make(chan Response, 1),
	}
	s.setResponseWaiter(waiter)
	defer s.clearResponseWaiter(waiter.ch)

	_, err := s.conn.Write(frame(data...))
	if err != nil {
		return nil, fmt.Errorf("sending command : %w", err)
	}

	timer := time.NewTimer(s.cmdTimeout)
	defer timer.Stop()

	for {
		select {
		case resp := <-waiter.ch:
			return &resp, nil
		case <-s.done:
			s.invalid.Store(true)
			return nil, ErrDisconnected
		case <-timer.C:
			// Timeout means request/response stream is desynchronized. Invalidate
			// this client instance and force callers to build a new one.
			s.invalid.Store(true)
			_ = s.conn.Close()
			return nil, ErrTimeout
		}
	}
}

func isExpectedResponse(requestCmd, responseCmd byte) bool {
	return responseCmd == ResponseStatusCmd || responseCmd == requestCmd
}

func (s *Satel) setResponseWaiter(w *responseWaiter) {
	s.waiterMu.Lock()
	defer s.waiterMu.Unlock()
	s.responseWaiter = w
}

func (s *Satel) getResponseWaiter() *responseWaiter {
	s.waiterMu.Lock()
	defer s.waiterMu.Unlock()
	return s.responseWaiter
}

func (s *Satel) clearResponseWaiter(ch chan Response) {
	s.waiterMu.Lock()
	defer s.waiterMu.Unlock()
	if s.responseWaiter != nil && s.responseWaiter.ch == ch {
		s.responseWaiter = nil
	}
}

// sendCmdWithResultCheck sends a command and expects a response status
// that indicates whether the command was successful or not.
func (s *Satel) sendCmdWithResultCheck(data []byte) error {
	resp, err := s.sendCmd(data...)
	if err != nil {
		return fmt.Errorf("sending command: %w", err)
	}

	if resp.cmd != ResponseStatusCmd {
		return fmt.Errorf("expected response status (0x%02X) but received for command 0x%02X: %w",
			ResponseStatusCmd, resp.cmd, ErrProtocolViolation,
		)
	}
	if resp.status.IsError() {
		return fmt.Errorf("error response status received %q", resp.status)
	}
	return nil
}

func (s *Satel) getSatelDeviceInfo() (device, string, language, error) {
	resp, err := s.sendCmd(SatelDeviceInfoCmd)
	if err != nil {
		return UnknownDevice, "", UnspecifiedLanguage, fmt.Errorf("getting satel device info: %w", err)
	}
	return decodeSatelDeviceInfo(resp.data...)
}
