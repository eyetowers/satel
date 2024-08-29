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

	UnknownDevice device = 255
)

var zoneAndOutputCapacity = map[device]uint64{
	INTEGRA24:           24,
	INTEGRA32:           32,
	INTEGRA64:           64,
	INTEGRA128:          128,
	INTEGRA128WRLSIM300: 128,
	INTEGRA128WRLLEON:   128,
	INTEGRA64Plus:       64,
	INTEGRA128Plus:      128,
	INTEGRA256Plus:      256,
}

var deviceModels = map[device]string{
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

func (d device) String() string {
	if deviceModels[d] == "" {
		return "Unknown Device"
	}

	return deviceModels[d]
}

func decodeSatelDeviceInfo(data ...byte) (device, string, error) {
	if len(data) != 14 {
		return UnknownDevice, "", fmt.Errorf("failed to decode device info %w", ErrCorruptedResponse)
	}
	model := device(data[0])
	data = data[1:]
	version := fmt.Sprintf("%s.%s %s-%s-%s", data[:1], data[1:3], data[3:7], data[7:9], data[9:11])
	return model, version, nil
}

func (d device) ZoneAndOutputCapacity() (uint64, error) {
	value, exists := zoneAndOutputCapacity[d]
	if !exists {
		return 0, fmt.Errorf("unknown device, zone and output capacity could not be determained")
	}

	return value, nil
}
