package satel

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var ErrDisconnected = errors.New("disconnected")
var ErrCrcNotMatch = errors.New("corrupt response: crc does not match")
var ErrCorruptedResponse = errors.New("corrupted response: does not match the documentation")

type Satel struct {
	conn     net.Conn
	userCode string
	mu       sync.Mutex
	cmdSize  int
	readChan chan byte
	resChan  chan Result
	Handler  Handler
}

func New(address, userCode string, h Handler) (*Satel, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connection to %s failed with error: %w", address, err)
	}

	if !isUserCodeValid(userCode) {
		return nil, fmt.Errorf("invalid user code")
	}
	return NewConfig(conn, userCode, h), nil
}

func NewConfig(conn net.Conn, userCode string, h Handler) *Satel {
	s := &Satel{
		conn:     conn,
		userCode: userCode,
		readChan: make(chan byte),
		resChan:  make(chan Result),
		Handler:  h,
		cmdSize:  16, // will have to change it later (Satel man, page 13)
	}

	go s.read()

	subscribedStates := []byte{0x7F, 0xFF, 0xFF, 0xFF, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	err := s.sendCmd(subscribedStates)
	if err != nil {
		return s
	}

	go s.keepConnectionAlive()
	return s
}

func (s *Satel) keepConnectionAlive() {
	for {
		err := s.sendReadCmd(0x7E)
		if err != nil {
			return
		}
		time.Sleep(5 * time.Second)
	}
}

func (s *Satel) ArmPartition(mode, index int) error {
	data := make([]byte, 4)
	data[index/8] = 1 << (index % 8)
	bytes := prepareCommand(byte(0x80+mode), s.userCode, data...)
	return s.sendCmd(bytes)
}

func (s *Satel) ForceArmPartition(mode, index int) error {
	data := make([]byte, 4)
	data[index/8] = 1 << (index % 8)
	bytes := prepareCommand(byte(0xA0+mode), s.userCode, data...)
	return s.sendCmd(bytes)
}

func (s *Satel) DisarmPartition(index int) error {
	data := make([]byte, 4)
	data[index/8] = 1 << (index % 8)
	bytes := prepareCommand(byte(0x84), s.userCode, data...)
	return s.sendCmd(bytes)
}

func (s *Satel) ClearAlarm(index int) error {
	data := make([]byte, 4)
	data[index/8] = 1 << (byte(index) % 8)
	bytes := prepareCommand(byte(0x85), s.userCode, data...)
	return s.sendCmd(bytes)
}

func (s *Satel) AlarmCheck(index int) error {
	data := make([]byte, 4)
	data[index/8] = 1 << (byte(index) % 8)
	bytes := prepareCommand(byte(0x13), s.userCode, data...)
	return s.sendCmd(bytes)
}

func (s *Satel) ZoneBypass(zone int) error {
	data := make([]byte, s.cmdSize)
	data[zone/8] = 1 << (byte(zone) % 8)
	bytes := prepareCommand(byte(0x86), s.userCode, data...)
	return s.sendCmd(bytes)
}

func (s *Satel) ZoneUnBypass(zone int) error {
	data := make([]byte, s.cmdSize)
	data[zone/8] = 1 << (byte(zone) % 8)
	bytes := prepareCommand(byte(0x87), s.userCode, data...)
	return s.sendCmd(bytes)
}

func (s *Satel) SetOutput(index int, value bool) error {
	cmd := byte(0x89)
	if value {
		cmd = 0x88
	}
	data := make([]byte, s.cmdSize)
	data[index/8] = 1 << (index % 8)
	bytes := prepareCommand(cmd, s.userCode, data...)
	return s.sendCmd(bytes)
}

func prepareCommand(cmd byte, userCode string, data ...byte) []byte {
	bytes := append([]byte{cmd}, transformCode(userCode)...)
	return append(bytes, data...)
}

func (s *Satel) Close() error {
	close(s.readChan)
	close(s.resChan)
	return s.conn.Close()
}

func decomposePayload(bytes ...byte) (byte, []byte, error) {
	const minByteLength = 3
	if len(bytes) < minByteLength {
		return 0, nil, ErrCorruptedResponse
	}
	cmd := bytes[0]
	dataWithCmd := bytes[:len(bytes)-2]
	data := bytes[1 : len(bytes)-2]
	crc := bytes[len(bytes)-2:]
	if !isCrcValid(dataWithCmd, crc) {
		return 0, nil, ErrCrcNotMatch
	}

	if cmd == 0xFE {
		return 0, nil, ErrCorruptedResponse
	}
	return cmd, data, nil
}

type command struct {
	prev        [32]byte
	initialized bool
}

func (s *Satel) read() {
	scanner := bufio.NewScanner(s.conn)
	scanner.Split(scan)
	commands := make(map[byte]command)
	defer s.Close()

	for {
		ok := scanner.Scan()
		if !ok {
			if scanner.Err() == nil {
				s.Handler.OnError(ErrDisconnected)
			} else {
				s.Handler.OnError(scanner.Err())
			}
			break
		}

		bytes := scanner.Bytes()
		cmd, data, err := decomposePayload(bytes...)
		if err != nil {
			s.Handler.OnError(err)
			break
		}

		if cmd == 0xEF {
			s.sendResponseStatus(data[0])
			continue
		}

		if cmd == 0x7E {
			s.sendVersionResponse(cmd)
			continue
		}

		c := commands[cmd]
		for i, bb := range data {
			change := bb ^ c.prev[i]
			for j := 0; j < 8; j++ {
				index := byte(1 << j)
				if !c.initialized || change&index != 0 {
					handleChange := handlerFunc(s.Handler, ChangeType(cmd))
					handleChange((i*8 + j), bb&index != 0)
				}
			}
			c.prev[i] = data[i]
		}
		c.initialized = true
		commands[cmd] = c
	}
}

func (s *Satel) sendVersionResponse(b byte) {
	select {
	case s.readChan <- b:
	default:
	}
}

func (s *Satel) sendResponseStatus(r byte) {
	select {
	case s.resChan <- Result(r):
	default:
	}
}

func (s *Satel) cmdResponseStatus() error {
	select {
	case r := <-s.resChan:
		if r.IsError() {
			return fmt.Errorf(r.String())
		}
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("timeout (3 seconds), no response")
	}
}

func (s *Satel) sendReadCmd(data ...byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return errors.New("no connection")
	}
	_, err := s.conn.Write(frame(data...))
	if err != nil {
		return err
	}

	select {
	case <-s.readChan:
	case <-time.After(3 * time.Second):
	}

	return nil
}

func (s *Satel) sendCmd(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return errors.New("no connection")
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
