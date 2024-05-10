package satel

import (
	"fmt"
)

type device byte

const (
	INTEGRA24           device = 0x00
	INTEGRA32           device = 0x01
	INTEGRA64           device = 0x02
	INTEGRA128          device = 0x03
	INTEGRA128WRLSIM300 device = 0x04
	INTEGRA128WRLLEON   device = 0x84
	INTEGRA64Plus       device = 0x42
	INTEGRA128Plus      device = 0x43
	INTEGRA256Plus      device = 0x48
)

func (d device) String() string {

	devices := map[device]string{
		INTEGRA24:           "INTEGRA 24",
		INTEGRA32:           "INTEGRA 32",
		INTEGRA64:           "INTEGRA 64",
		INTEGRA128:          "INTEGRA 128",
		INTEGRA128WRLSIM300: "INTEGRA 128-WRL SIM300",
		INTEGRA128WRLLEON:   "INTEGRA 128-WRL LEON",
		INTEGRA64Plus:       "INTEGRA 64 Plus",
		INTEGRA128Plus:      "INTEGRA 128 Plus",
		INTEGRA256Plus:      "INTEGRA 256 Plus",
	}

	if devices[d] == "" {
		return "invalid device"
	}

	return devices[d]
}

func decodeDeviceInfo(data ...byte) (string, string) {
	model := device(data[0]).String()
	data = data[1:]
	version := fmt.Sprintf("%s.%s %s-%s-%s", data[:1], data[1:3], data[3:7], data[7:9], data[9:])
	return model, version
}
