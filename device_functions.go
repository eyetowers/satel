package satel

// OutputFunction describes how an output is used (e.g. mono/bi switch).
type OutputFunction byte

const (
	NotUsed    OutputFunction = 0x00
	MonoSwitch OutputFunction = 0x18
	BiSwitch   OutputFunction = 0x19
)
