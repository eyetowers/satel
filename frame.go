package satel

import (
	"math/bits"
)

const (
	crcSeed uint16 = 0x147A

	reserveBytes int = 4
	crcBytes     int = 2
	cmdBytes     int = 1
)

func frame(data ...byte) []byte {
	preamble := []byte{0xFE, 0xFE}
	postamble := []byte{0xFE, 0x0D}

	buf := make([]byte, 0, (cmdBytes + len(data) + crcBytes + cmdBytes + reserveBytes))
	buf = append(buf, preamble...)
	buf = appendWithSpecialByte(buf, data...)
	buf = appendWithSpecialByte(buf, crc(data)...)

	return append(buf, postamble...)

}

func crc(data []byte) []byte {
	c := crcSeed
	for _, b := range data {
		c = update(c, b)
	}
	return []byte{byte(c >> 8), byte(c & 0xFF)}
}

func update(c uint16, b byte) uint16 {
	c = bits.RotateLeft16(c, 1)
	c ^= 0xFFFF
	return c + c>>8 + uint16(b)
}

func appendWithSpecialByte(buf []byte, data ...byte) []byte {
	for _, b := range data {
		buf = append(buf, b)
		if b == 0xFE {
			buf = append(buf, 0xF0)
		}
	}
	return buf
}

func isCrcValid(data []byte, targetCrc []byte) bool {
	validCrc := crc(data)

	for i := range targetCrc {
		if validCrc[i] != targetCrc[i] {
			return false
		}
	}

	return true
}
