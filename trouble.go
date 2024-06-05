package satel

type Trouble3Type int

const (
	ACU100ModuleJam Trouble3Type = iota
	LowBattery
	DeviceNoCommunication
	OutputNoCommunication
)
