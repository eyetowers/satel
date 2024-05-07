package satel

import (
	"bytes"
	"fmt"
)

// scan finds the actual response removing command prefix and postfix
func scan(data []byte, _ bool) (advance int, token []byte, err error) {
	for _, d := range data {
		fmt.Printf("0x%0X ", d)
	}
	println("EOL")

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
	// fmt.Println(index)
	if index > 0 {
		filteredData, err := removeSpecialByte(data[:index])
		if err != nil {
			return 0, nil, err
		}

		return i + index + 2, filteredData, nil
	}

	return 0, nil, nil
}

func removeSpecialByte(bytes []byte) ([]byte, error) {
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
