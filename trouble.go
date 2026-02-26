package satel

// Trouble3Type is the trouble category for OnTroublePart3 (e.g. low battery, no communication).
type Trouble3Type int

const (
	ACU100ModuleJam Trouble3Type = iota
	LowBattery
	DeviceNoCommunication
	OutputNoCommunication
)
