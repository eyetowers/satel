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

const keepAliveCmd = 5

type Satel struct {
	conn         net.Conn
	usercode     []byte
	mu           sync.Mutex
	cmdSize      int
	versionChan  chan []byte
	responseChan chan Result
	handler      Handler
	closing      atomic.Bool
	done         chan bool
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
		versionChan:  make(chan []byte),
		responseChan: make(chan Result),
		handler:      h,
		cmdSize:      16,
		done:         make(chan bool),
	}

	go s.read()

	model, version, err := s.getDeviceInfo()
	if err != nil {
		return nil, err
	}
	if version[0] == 0x32 && model == INTEGRA256Plus.String() {
		s.cmdSize = 32
	}

	go s.keepConnectionAlive()

	subscribedStates := subsStates() // TODO, give this control to the user.
	err = s.sendCmd(subscribe(subscribedStates...))
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Satel) keepConnectionAlive() {
	for {
		err := s.sendReadCmd(0x7E)
		if err != nil {
			return
		}
		time.Sleep(keepAliveCmd * time.Second)
	}
}

func (s *Satel) ArmPartition(mode, index int) error {
	bytes := s.prepareCommand(byte(0x80+mode), 4, index)
	return s.sendCmd(bytes)
}

func (s *Satel) ForceArmPartition(mode, index int) error {
	bytes := s.prepareCommand(byte(0xA0+mode), 4, index)
	return s.sendCmd(bytes)
}

func (s *Satel) DisarmPartition(index int) error {
	bytes := s.prepareCommand(byte(0x84), 4, index)
	return s.sendCmd(bytes)
}

func (s *Satel) ClearAlarm(index int) error {
	bytes := s.prepareCommand(byte(0x85), 4, index)
	return s.sendCmd(bytes)
}

func (s *Satel) AlarmCheck(index int) error {
	bytes := s.prepareCommand(byte(0x13), 4, index)
	return s.sendCmd(bytes)
}

func (s *Satel) ZoneBypass(zone int) error {
	bytes := s.prepareCommand(byte(0x86), s.cmdSize, zone)
	return s.sendCmd(bytes)
}

func (s *Satel) ZoneUnBypass(zone int) error {
	bytes := s.prepareCommand(byte(0x87), s.cmdSize, zone)
	return s.sendCmd(bytes)
}

func (s *Satel) SetOutput(index int, value bool) error {
	cmd := byte(0x89)
	if value {
		cmd = 0x88
	}
	bytes := s.prepareCommand(cmd, s.cmdSize, index)
	return s.sendCmd(bytes)
}

func (s *Satel) ClearTroubleMemory() error {
	bytes := append([]byte{0x8B}, s.usercode...)
	return s.sendCmd(bytes)
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

func decomposePayload(bytes ...byte) (byte, []byte, error) {
	const minByteLength = 3
	if len(bytes) < minByteLength {
		return 0, nil, ErrCorruptedResponse
	}
	crcIndex := len(bytes) - 2
	crc := bytes[crcIndex:]
	cmd := bytes[0]
	dataWithCmd := bytes[:crcIndex]
	data := dataWithCmd[1:]

	if !isCrcValid(dataWithCmd, crc) {
		return 0, nil, ErrCrcNotMatch
	}

	if cmd == 0xFE {
		return 0, nil, ErrForbiddenCommand
	}
	return cmd, data, nil
}

func (s *Satel) closeRead() {
	close(s.versionChan)
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

		if cmd == 0xEF {
			s.sendResponseStatus(data[0])
			continue
		}

		if cmd == 0x7E {
			s.sendVersionResponse(data...)
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
	troubleType := Trouble3Type(i / 15)
	idx := (((i - (int(troubleType) * 15)) * 8) + (8 - j))
	s.handler.OnTroublePart3(idx, troubleType, bb&index != 0, !c.initialized)
}

func (s *Satel) sendVersionResponse(data ...byte) {
	select {
	case s.versionChan <- data:
	default:
	}
}

func (s *Satel) sendResponseStatus(r byte) {
	select {
	case s.responseChan <- Result(r):
	default:
	}
}

func (s *Satel) cmdResponseStatus() error {
	select {
	case r := <-s.responseChan:
		if r.IsError() {
			return fmt.Errorf(r.String())
		}
		return nil
	case <-time.After(3 * time.Second):
		return ErrTimeout
	}
}

func (s *Satel) getDeviceInfo() (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cmd := byte(0x7E)
	if s.conn == nil {
		return "", "", ErrNoConnection
	}
	_, err := s.conn.Write(frame(cmd))
	if err != nil {
		return "", "", err
	}

	select {
	case r := <-s.versionChan:
		model, version, err := decodeDeviceInfo(r...)
		return model, version, err
	case <-time.After(3 * time.Second):
		return "", "", ErrTimeout
	}
}

func (s *Satel) sendReadCmd(cmd ...byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return ErrNoConnection
	}
	_, err := s.conn.Write(frame(cmd...))
	if err != nil {
		return err
	}

	select {
	case <-s.versionChan:
	case <-time.After(3 * time.Second):
	}
	return nil
}

func (s *Satel) sendCmd(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return ErrNoConnection
	}
	_, err := s.conn.Write(frame(data...))
	if err != nil {
		return err
	}
	return s.cmdResponseStatus()
}

func transformCode(code string) []byte {
	bytes := make([]byte, 8)
	for i := 0; i < 16; i++ {
		if i < len(code) {
			digit := code[i]
			if i%2 == 0 {
				bytes[i/2] = (digit - '0') << 4
			} else {
				bytes[i/2] |= digit - '0'
			}
		} else if i%2 == 0 {
			bytes[i/2] = 0xFF
		} else if i == len(code) {
			bytes[i/2] |= 0x0F
		}
	}
	return bytes
}
