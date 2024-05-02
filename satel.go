package satel

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var ErrCrcNotMatch = errors.New("corrupt response: crc does not match")

type Satel struct {
	conn     net.Conn
	userCode string
	mu       sync.Mutex
	cmdSize  int
	cmdChan  chan int
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
		cmdChan:  make(chan int),
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

	return s
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

	return s.conn.Close()

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
			if scanner.Err() == ErrBusy {
				s.Handler.OnError(ErrBusy)
			}
			break
		}

		bytes := scanner.Bytes()
		cmd := bytes[0]

		if cmd == 0xFE {
			// cmd cannot be 0xFE
			continue
		}

		if !isCrcValid(bytes[:len(bytes)-2], bytes[len(bytes)-2:]) {
			s.Handler.OnError(ErrCrcNotMatch)
		}

		bytes = bytes[1 : len(bytes)-2]
		s.cmdRes()
		if cmd == 0xEF {
			s.resChan <- Result(bytes[0])
			continue
		}
		c := commands[cmd]
		for i, bb := range bytes {
			change := bb ^ c.prev[i]
			for j := 0; j < 8; j++ {
				index := byte(1 << j)
				if !c.initialized || change&index != 0 {
					handleChange := handlerFunc(s.Handler, ChangeType(cmd))
					handleChange((i*8 + j), bb&index != 0)
				}
			}
			c.prev[i] = bytes[i]
		}
		c.initialized = true
		commands[cmd] = c
	}
}

func (s *Satel) cmdRes() {
	select {
	case s.cmdChan <- 0:
	default:
	}
}

func (s *Satel) sendCmd(data []byte, readOnlyCmd ...bool) error {
	if len(readOnlyCmd) > 1 {
		return fmt.Errorf("only one boolean argument allowed")
	}

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
	case <-s.cmdChan:
	case <-time.After(3 * time.Second):
	}

	if len(readOnlyCmd) > 0 && readOnlyCmd[0] {
		return nil
	}

	r, ok := <-s.resChan
	if !ok {
		return fmt.Errorf("failed to listen to command response")
	}

	if r.IsError() {
		return fmt.Errorf(r.String())
	}

	return nil
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
