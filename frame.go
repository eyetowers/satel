package satel

import "math/bits"

const seed uint16 = 0x147A

func frame(data ...byte) []byte {
	crc := crc(data)
	f := append(data, crc...)
	f = replaceSpecialByte(f)
	f = append([]byte{0xFE, 0xFE}, f...)
	return append(f, 0xFE, 0x0D)
}

func crc(data []byte) []byte {
	c := seed
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

func replaceSpecialByte(input []byte) []byte {
	var output []byte
	for i := 0; i < len(input); i++ {
		if input[i] == 0xFE {
			output = append(output, 0xFE, 0xF0)
		} else {
			output = append(output, input[i])
		}
	}
	return output
}
