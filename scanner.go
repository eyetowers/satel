package satel

import (
	"bytes"
	"errors"
)

var firstbytes = true

var ErrBusy = errors.New("busy connection")

// scan finds the actual response removing command prefix and postfix
func scan(data []byte, _ bool) (advance int, token []byte, err error) {
	if firstbytes {
		firstbytes = false
		if IsBusy(data...) {
			return 0, nil, ErrBusy
		}
	}

	i := 0
	for ; i < len(data) && data[i] == 0xFE; i++ {
	}
	if i > 0 {
		data = data[i:]
	}
	startIndex := bytes.Index(data, []byte{0xFE, 0xFE})
	index := bytes.Index(data, []byte{0xFE, 0x0D})
	if startIndex > 0 && (index < 0 || startIndex < index) {
		return i + startIndex + 2, nil, nil
	}
	if index > 0 {
		return i + index + 2, data[:index], nil
	}
	return 0, nil, nil
}

func IsBusy(bytes ...byte) bool {
	expected := []byte{0x10, 0x42, 0x75, 0x73, 0x79, 0x21, 0x0D, 0x0A}

	if len(bytes) < len(expected) {
		return false
	}

	for i := 0; i < len(expected); i++ {
		if bytes[i] != expected[i] {
			return false
		}
	}
	return true
}
