package satel

import (
	"errors"
	"strings"
)

var ErrInvalidChar = errors.New("usercode contains invalid character")
var ErrInvalidLength = errors.New("usercode does not match the expected length")

const subscribeCmd = 0x7F

func validateUsercode(usercode string) error {
	if len(usercode) != 4 {
		return ErrInvalidLength
	}

	for _, char := range usercode {
		if char < '0' || char > '9' {
			return ErrInvalidChar
		}
	}
	return nil
}

func transformSubscription(bytes ...StateType) []byte {
	payload := make([]byte, 13)
	payload[0] = subscribeCmd
	for _, b := range bytes {
		pos := b/8 + 1
		payload[pos] |= 1 << (b % 8)
	}
	return payload
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

func decodePartition(data []byte) (byte, uint64, string) {
	deviceType := data[0]
	partitionID := data[1]
	name := string(data[3:])
	return deviceType, uint64(partitionID), strings.TrimSpace(name)
}

func decodeZone(data []byte) (byte, uint64, string, uint64) {
	deviceType := data[0]
	zoneID := data[1]
	name := string(data[3 : len(data)-1])
	partition := data[len(data)-1]
	return deviceType, uint64(zoneID), strings.TrimSpace(name), uint64(partition)
}

func decodeOutput(data []byte) (byte, uint64, string) {
	deviceType := data[0]
	outputID := data[1]
	name := string(data[3:])
	return deviceType, uint64(outputID), strings.TrimSpace(name)
}
