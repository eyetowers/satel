package satel

type Handler interface {
	OnZoneViolations(index int, state, initial bool)
	OnZoneTamper(index int, state, initial bool)
	OnZoneAlarm(index int, state, initial bool)
	OnZoneTamperAlarm(index int, state, initial bool)
	OnZoneAlarmMemory(index int, state, initial bool)
	OnZoneTamperAlarmMemory(index int, state, initial bool)
	OnZoneBypass(index int, state, initial bool)
	OnZoneNoViolationTrouble(index int, state, initial bool)
	OnZoneLongViolationTrouble(index int, state, initial bool)
	OnArmedPartitionSuppressed(index int, state, initial bool)
	OnArmedPartition(index int, state, initial bool)
	OnPartitionArmedInMode2(index int, state, initial bool)
	OnPartitionArmedInMode3(index int, state, initial bool)
	OnPartitionWith1stCodeEntered(index int, state, initial bool)
	OnPartitionEntryTime(index int, state, initial bool)
	OnPartitionExitTimeOver10s(index int, state, initial bool)
	OnPartitionExitTimeUnder10s(index int, state, initial bool)
	OnPartitionTemporaryBlocked(index int, state, initial bool)
	OnPartitionBlockedForGuardRound(index int, state, initial bool)
	OnPartitionAlarm(index int, state, initial bool)
	OnPartitionFireAlarm(index int, state, initial bool)
	OnPartitionAlarmMemory(index int, state, initial bool)
	OnPartitionFireAlarmMemory(index int, state, initial bool)
	OnOutput(index int, state, initial bool)
	OnDoorOpened(index int, state, initial bool)
	OnDoorOpenedLong(index int, state, initial bool)
	OnStatusBit(index int, state, initial bool)
	OnTroublePart1(index int, state, initial bool)
	OnTroublePart2(index int, state, initial bool)
	OnTroublePart4(index int, state, initial bool)
	OnTroublePart5(index int, state, initial bool)
	OnTroubleMemoryPart1(index int, state, initial bool)
	OnTroubleMemoryPart2(index int, state, initial bool)
	OnTroubleMemoryPart3(index int, state, initial bool)
	OnTroubleMemoryPart4(index int, state, initial bool)
	OnTroubleMemoryPart5(index int, state, initial bool)
	OnPartitionWithViolatedZones(index int, state, initial bool)
	OnZoneIsolate(index int, state, initial bool)

	OnTroublePart3(index int, troubleType Trouble3Type, trouble, initial bool)
	OnError(err error)
}

func handlerFunc(h Handler, cmd StateType) func(int, bool, bool) {
	functions := [...]func(int, bool, bool){
		h.OnZoneViolations,
		h.OnZoneTamper,
		h.OnZoneAlarm,
		h.OnZoneTamperAlarm,
		h.OnZoneAlarmMemory,
		h.OnZoneTamperAlarmMemory,
		h.OnZoneBypass,
		h.OnZoneNoViolationTrouble,
		h.OnZoneLongViolationTrouble,
		h.OnArmedPartitionSuppressed,
		h.OnArmedPartition,
		h.OnPartitionArmedInMode2,
		h.OnPartitionArmedInMode3,
		h.OnPartitionWith1stCodeEntered,
		h.OnPartitionEntryTime,
		h.OnPartitionExitTimeOver10s,
		h.OnPartitionExitTimeUnder10s,
		h.OnPartitionTemporaryBlocked,
		h.OnPartitionBlockedForGuardRound,
		h.OnPartitionAlarm,
		h.OnPartitionFireAlarm,
		h.OnPartitionAlarmMemory,
		h.OnPartitionFireAlarmMemory,
		h.OnOutput,
		h.OnDoorOpened,
		h.OnDoorOpenedLong,
		h.OnStatusBit,
		h.OnTroublePart1,
		h.OnTroublePart2,
		h.OnTroublePart4,
		h.OnTroublePart5,
		h.OnTroubleMemoryPart1,
		h.OnTroubleMemoryPart2,
		h.OnTroubleMemoryPart3,
		h.OnTroubleMemoryPart4,
		h.OnTroubleMemoryPart5,
		h.OnPartitionWithViolatedZones,
		h.OnZoneIsolate,
	}

	return functions[cmd]
}

// IgnoreHandler implements empty Handler functions. Use this to ignore the Handler functions.
// Overwrite what you want to use.
type IgnoreHandler struct {
}

func (IgnoreHandler) OnZoneViolations(index int, state, initial bool)                {}
func (IgnoreHandler) OnZoneTamper(index int, state, initial bool)                    {}
func (IgnoreHandler) OnZoneAlarm(index int, state, initial bool)                     {}
func (IgnoreHandler) OnZoneTamperAlarm(index int, state, initial bool)               {}
func (IgnoreHandler) OnZoneAlarmMemory(index int, state, initial bool)               {}
func (IgnoreHandler) OnZoneTamperAlarmMemory(index int, state, initial bool)         {}
func (IgnoreHandler) OnZoneBypass(index int, state, initial bool)                    {}
func (IgnoreHandler) OnZoneNoViolationTrouble(index int, state, initial bool)        {}
func (IgnoreHandler) OnZoneLongViolationTrouble(index int, state, initial bool)      {}
func (IgnoreHandler) OnArmedPartitionSuppressed(index int, state, initial bool)      {}
func (IgnoreHandler) OnArmedPartition(index int, state, initial bool)                {}
func (IgnoreHandler) OnPartitionArmedInMode2(index int, state, initial bool)         {}
func (IgnoreHandler) OnPartitionArmedInMode3(index int, state, initial bool)         {}
func (IgnoreHandler) OnPartitionWith1stCodeEntered(index int, state, initial bool)   {}
func (IgnoreHandler) OnPartitionEntryTime(index int, state, initial bool)            {}
func (IgnoreHandler) OnPartitionExitTimeOver10s(index int, state, initial bool)      {}
func (IgnoreHandler) OnPartitionExitTimeUnder10s(index int, state, initial bool)     {}
func (IgnoreHandler) OnPartitionTemporaryBlocked(index int, state, initial bool)     {}
func (IgnoreHandler) OnPartitionBlockedForGuardRound(index int, state, initial bool) {}
func (IgnoreHandler) OnPartitionAlarm(index int, state, initial bool)                {}
func (IgnoreHandler) OnPartitionFireAlarm(index int, state, initial bool)            {}
func (IgnoreHandler) OnPartitionAlarmMemory(index int, state, initial bool)          {}
func (IgnoreHandler) OnPartitionFireAlarmMemory(index int, state, initial bool)      {}
func (IgnoreHandler) OnOutput(index int, state, initial bool)                        {}
func (IgnoreHandler) OnDoorOpened(index int, state, initial bool)                    {}
func (IgnoreHandler) OnDoorOpenedLong(index int, state, initial bool)                {}
func (IgnoreHandler) OnStatusBit(index int, state, initial bool)                     {}
func (IgnoreHandler) OnTroublePart1(index int, state, initial bool)                  {}
func (IgnoreHandler) OnTroublePart2(index int, state, initial bool)                  {}
func (IgnoreHandler) OnTroublePart4(index int, state, initial bool)                  {}
func (IgnoreHandler) OnTroublePart5(index int, state, initial bool)                  {}
func (IgnoreHandler) OnTroubleMemoryPart1(index int, state, initial bool)            {}
func (IgnoreHandler) OnTroubleMemoryPart2(index int, state, initial bool)            {}
func (IgnoreHandler) OnTroubleMemoryPart3(index int, state, initial bool)            {}
func (IgnoreHandler) OnTroubleMemoryPart4(index int, state, initial bool)            {}
func (IgnoreHandler) OnTroubleMemoryPart5(index int, state, initial bool)            {}
func (IgnoreHandler) OnPartitionWithViolatedZones(index int, state, initial bool)    {}
func (IgnoreHandler) OnZoneIsolate(index int, state, initial bool)                   {}

func (IgnoreHandler) OnTroublePart3(index int, troubleType Trouble3Type, trouble, initial bool) {}
