package satel

import (
	"bytes"
)

var (
	preamble  = []byte{0xFE, 0xFE}
	postamble = []byte{0xFE, 0x0D}
)

// scan finds the actual response removing command prefix and postfix
func scan(data []byte, _ bool) (advance int, token []byte, err error) {
	end := bytes.Index(data, postamble)
	if end < 0 {
		// No complete packet yet, wait for more bytes.
		return 0, nil, nil
	}
	data = data[:end]

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
