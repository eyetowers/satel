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
var ErrTimeout = errors.New("timeout (3 seconds), no response")
var ErrNoConnection = errors.New("no connection")
var ErrReturnResponse = errors.New("failed returning response. unexpectly buffer full")

const (
	KeepAliveInterval = 5 * time.Second
	CmdTimeOut        = 3 * time.Second

	ResponseStatusCmd = byte(0xEF)
	VersionStatusCmd  = byte(0x7E)
	DeviceInfoCmd     = byte(0x7E)
	ReadDeviceCmd     = byte(0xEE)
)

type Satel struct {
	conn     net.Conn
	usercode []byte
	mu       sync.Mutex
	cmdSize  int

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

func New(address, usercode string, h Handler) (*Satel, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connection to %s failed with error: %w", address, err)
	}

	err = validateUsercode(usercode)
	if err != nil {
		return nil, err
	}

	return newConfig(conn, usercode, h)
}

func newConfig(conn net.Conn, usercode string, h Handler) (*Satel, error) {
	s := &Satel{
		conn:         conn,
		usercode:     transformCode(usercode),
		responseChan: make(chan Response),
		handler:      h,
		cmdSize:      16,
		done:         make(chan bool),
	}

	go s.read()

	model, version, err := s.getDeviceInfo()
	if err != nil {
		return nil, err
	}
	if version[0] == '2' && model == INTEGRA256Plus.String() {
		s.cmdSize = 32
	}

	go s.keepConnectionAlive()

	return s, nil
}

func (s *Satel) keepConnectionAlive() {
	for {
		_, err := s.sendCmd(DeviceInfoCmd)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		time.Sleep(KeepAliveInterval)
	}
}

func (s *Satel) GetZones() ([]Zone, error) {
	// TODO @tsaikat: need to dynamically select possible zones.
	possibleZones := 32
	cmd := ReadDeviceCmd
	typeZone := byte(0x05)
	expectedResposeSize := 20

	var zones []Zone
	partitions := make(map[uint64]Partition)
	for i := 1; i < possibleZones; i++ {
		resp, err := s.sendCmd(cmd, typeZone, byte(i))
		if err != nil {
			return nil, err
		}

		if resp.cmd != cmd && resp.cmd != ResponseStatusCmd {
			return nil, fmt.Errorf("unexpected response recieved while getting zone(%d) information", i)
		}

		if resp.cmd == ResponseStatusCmd {
			if !resp.status.IsError() {
				return nil, fmt.Errorf("getting zone (%d).expected error", i)
			}
			continue
		}

		if len(resp.data) != expectedResposeSize {
			return nil, fmt.Errorf("getting zone information. payload size(%d) does not match expected payload size(%d)",
				len(resp.data), expectedResposeSize,
			)
		}

		deviceType, zoneID, name, partitionID := decodeZone(resp.data)
		if typeZone != deviceType {
			return nil, fmt.Errorf("unexpected. payload received for wrong type")
		}

		partition, exist := partitions[partitionID]
		if !exist {
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
	cmd := ReadDeviceCmd
	typePartition := byte(0x00)
	expectedResposeSize := 19

	resp, err := s.sendCmd(cmd, typePartition, byte(partition))
	if err != nil {
		return Partition{}, err
	}

	if resp.cmd != cmd && resp.cmd != ResponseStatusCmd {
		return Partition{}, fmt.Errorf("unexpected response recieved while getting partition(%d) information", partition)
	}

	if resp.cmd == ResponseStatusCmd {
		return Partition{}, fmt.Errorf("unexpected. getting partition information of a zone failed")
	}

	if len(resp.data) != expectedResposeSize {
		return Partition{}, fmt.Errorf("getting partition information. payload size(%d) does not match expected payload size(%d)",
			len(resp.data), expectedResposeSize,
		)
	}

	deviceType, partitionID, partitionName := decodePartition(resp.data)
	if typePartition != deviceType {
		return Partition{}, fmt.Errorf("unexpected. payload received for wrong type")
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

func (s *Satel) ArmPartition(mode, index int) error {
	bytes := s.prepareCommand(byte(0x80+mode), 4, index)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) ForceArmPartition(mode, index int) error {
	bytes := s.prepareCommand(byte(0xA0+mode), 4, index)
	return s.sendCmdWithResultCheck(bytes)
}

func (s *Satel) DisarmPartition(index int) error {
	bytes := s.prepareCommand(byte(0x84), 4, index)
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
	data := make([]byte, cmdSize)
	data[index/8] = 1 << (index % 8)
	bytes := append([]byte{cmd}, s.usercode...)
	return append(bytes, data...)
}

func (s *Satel) Close() error {
	s.closing.Store(true)
	err := s.conn.Close()
	<-s.done
	return err
}

func (s *Satel) closeRead() {
	close(s.responseChan)
	s.conn.Close()
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

		if cmd == ResponseStatusCmd || cmd == VersionStatusCmd || cmd == ReadDeviceCmd {
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
						// Adding 1 to index since Satel device index starts at 1 instead of 0.
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
		s.handler.OnError(ErrReturnResponse)
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
		return nil, err
	}

	select {
	case resp := <-s.responseChan:
		return &resp, nil
	case <-time.After(CmdTimeOut):
		return nil, ErrTimeout
	}
}

// sendCmdWithResultCheck sends a command and expects a response status
// that indicates whether the command was successful or not.
func (s *Satel) sendCmdWithResultCheck(data []byte) error {
	resp, err := s.sendCmd(data...)
	if err != nil {
		return err
	}

	if resp.cmd != ResponseStatusCmd {
		return fmt.Errorf("expected response status (0x%02X) but received for command 0x%02X: %w",
			ResponseStatusCmd, resp.cmd, ErrCorruptedResponse,
		)
		// TODO: need to think this through. the connection is legging. unexpected happen. shall we restart connection?
	}
	if resp.status.IsError() {
		return fmt.Errorf(resp.status.String())
	}
	return nil
}

func (s *Satel) getDeviceInfo() (string, string, error) {
	resp, err := s.sendCmd(DeviceInfoCmd)
	if err != nil {
		return "", "", err
	}
	return decodeDeviceInfo(resp.data...)
}
