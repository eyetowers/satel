package satel

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var ErrDisconnected = errors.New("disconnected")
var ErrCrcNotMatch = errors.New("corrupt response: crc does not match")
var ErrCorruptedResponse = errors.New("corrupted response: does not match the documentation")
var ErrForbiddenCommand = errors.New("forbidden command value")
var ErrNoConnection = errors.New("no connection")
var ErrReturnResponse = errors.New("failed returning response, unexpectly no caller available")
var ErrProtocolViolation = errors.New("response violates protocol")
var ErrDeviceNotFound = errors.New("requested device not found")
var ErrTimeout = fmt.Errorf("timeout (%s), no response", CmdTimeout.String())

const (
	KeepAliveInterval = 20 * time.Second
	CmdTimeout        = 10 * time.Second

	ResponseStatusCmd  = byte(0xEF)
	SatelDeviceInfoCmd = byte(0x7E)
	SatelDeviceVersion = byte(0x7C)
	ReadDeviceCmd      = byte(0xEE)
)

type Satel struct {
	conn               net.Conn
	usercode           []byte
	mu                 sync.Mutex
	cmdSize            int
	zoneOutputCapacity uint64
	deviceLanguage     language

	responseChan chan Response
	handler      Handler
	closing      atomic.Bool
	done         chan bool
}

type Response struct {
	cmd    byte
	data   []byte
	status ResponseStatus
}

type Zone struct {
	ID        uint64
	Name      string
	Partition Partition
}

type Partition struct {
	ID   uint64
	Name string
}

type Output struct {
	ID       uint64
	Name     string
	Function OutputFunction
}

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

	return newConfig(conn, usercode, h)
}

func newConfig(conn net.Conn, usercode string, h Handler) (*Satel, error) {
	s := &Satel{
		conn:               conn,
		usercode:           transformCode(usercode),
		responseChan:       make(chan Response),
		handler:            h,
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
	for {
		// Sending this random command just to keep the connection alive.
		_, err := s.sendCmd(SatelDeviceVersion)
		if err != nil {
			s.reportError(fmt.Errorf("keeping connection alive: %w", err))
			return
		}
		time.Sleep(KeepAliveInterval)
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

// Subscribe will subscribe to `states` StateType.
// This will activate Satel to send updates on any changed data on `states` StateType.
func (s *Satel) Subscribe(states ...StateType) error {
	err := s.sendCmdWithResultCheck(transformSubscription(states...))
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}
	return nil
}

func (s *Satel) ArmPartition(mode, partition int) error {
	bytes := s.prepareCommand(byte(0x80+mode), 4, partition)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) ArmPartitionNoDelay(mode, partition int) error {
	bytes := s.prepareCommand(byte(0x80+mode), 4, partition)
	bytes = append(bytes, 0x80)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) ForceArmPartition(mode, partition int) error {
	bytes := s.prepareCommand(byte(0xA0+mode), 4, partition)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) ForceArmPartitionNoDelay(mode, partition int) error {
	bytes := s.prepareCommand(byte(0xA0+mode), 4, partition)
	bytes = append(bytes, 0x80)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) DisarmPartition(partition int) error {
	bytes := s.prepareCommand(byte(0x84), 4, partition)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) ClearAlarm(index int) error {
	bytes := s.prepareCommand(byte(0x85), 4, index)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) AlarmCheck(index int) error {
	bytes := s.prepareCommand(byte(0x13), 4, index)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) ZoneBypass(zone int) error {
	bytes := s.prepareCommand(byte(0x86), s.cmdSize, zone)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) ZoneUnBypass(zone int) error {
	bytes := s.prepareCommand(byte(0x87), s.cmdSize, zone)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) SetOutput(index int, value bool) error {
	cmd := byte(0x89)
	if value {
		cmd = 0x88
	}
	bytes := s.prepareCommand(cmd, s.cmdSize, index)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) ClearTroubleMemory() error {
	bytes := append([]byte{0x8B}, s.usercode...)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) prepareCommand(cmd byte, cmdSize int, index int) []byte {
	// Substracting 1 from index since Satel indexes from 0.
	index = index - 1
	data := make([]byte, cmdSize)
	data[index/8] = 1 << (index % 8)
	bytes := append([]byte{cmd}, s.usercode...)
	return append(bytes, data...)
}

func (s *Satel) Close() error {
	s.closing.Store(true)
	err := s.conn.Close()
	<-s.done
	return fmt.Errorf("closing satel connection: %w", err)
}

func (s *Satel) closeRead() {
	close(s.responseChan)
	_ = s.conn.Close()
	close(s.done)
}

func (s *Satel) reportError(err error) {
	if !s.closing.Load() {
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

		c := commands[cmd]
		for i, bb := range data {
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
		resp.data = data
	}

	select {
	case s.responseChan <- resp:
	default:
		s.reportError(ErrReturnResponse)
	}
}

func (s *Satel) sendCmd(data ...byte) (*Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return nil, ErrNoConnection
	}
	_, err := s.conn.Write(frame(data...))
	if err != nil {
		return nil, fmt.Errorf("sending command : %w", err)
	}

	select {
	case resp := <-s.responseChan:
		return &resp, nil
	case <-time.After(CmdTimeout):
		return nil, ErrTimeout
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
