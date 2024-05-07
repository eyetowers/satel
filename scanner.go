package satel

import (
	"bytes"
	"fmt"
)

var (
	preamble  = []byte{0xFE, 0xFE}
	postamble = []byte{0xFE, 0x0D}
)

// scan finds the actual response removing command prefix and postfix
func scan(data []byte, _ bool) (advance int, token []byte, err error) {
	end := findEnd(data)
	if end < 0 {
		// No complete packet yet, wait for more bytes.
		return 0, nil, nil
	}

	for _, d := range data {
		fmt.Printf("0x%02X ", d)
	}
	println()
	data = data[:end]

	for _, d := range data {
		fmt.Printf("0x%02X ", d)
	}

	start := bytes.LastIndex(data, preamble)
	if start < 0 {
		// Packet end without a preamble, fail reading.
		return 0, nil, ErrCorruptedResponse
	}
	data = data[start+2:]

	payload, err := unescape(data)
	if err != nil {
		return 0, nil, err
	}
	return end + 2, payload, nil
}

func findEnd(data []byte) int {
	i := 0
	for {
		found := bytes.Index(data[i:], postamble)
		if found < 0 {
			return -1
		}
		end := found + i
		if end == 0 {
			// Found at the begining, corrupt message.
			return end + i
		}
		if data[end-1] != 0xFE {
			// The previous byte is not 0xFE, so it is an actual match.
			return end
		}
		// Skip and find next.
		i += found + 2
	}
}

func unescape(bytes []byte) ([]byte, error) {
	i := 0
	j := 0
	for ; i < len(bytes); i++ {
		bytes[j] = bytes[i]

		if bytes[i] == 0xFE {
			i++
			if i >= len(bytes) || bytes[i] != 0xF0 {
				return nil, ErrCorruptedResponse
			}
		}
		j++
	}

	return bytes[:j], nil
}
