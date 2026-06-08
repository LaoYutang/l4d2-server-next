//go:generate go run golang.org/x/tools/cmd/stringer -type Type -trimprefix Type

package demo

const Magic uint64 = 0x4f4d4544324c48 // HL2DEMO

type Header struct {
	Magic           uint64 // should equal Magic
	Protocol        int32  // should equal 4
	NetworkProtocol int32
	ServerName      [260]byte
	ClientName      [260]byte
	MapName         [260]byte
	GameDirectory   [260]byte
	PlaybackTime    float32
	PlaybackTicks   int32
	PlaybackFrames  int32
	SignonLength    int32
}

type Type uint8

const (
	TypeSignon       Type = 1
	TypePacket       Type = 2
	TypeSyncTick     Type = 3
	TypeConsoleCmd   Type = 4
	TypeUserCmd      Type = 5
	TypeDataTables   Type = 6
	TypeStop         Type = 7
	TypeCustomData   Type = 8
	TypeStringTables Type = 9
)

type Command struct {
	Type       Type
	Tick       int32
	PlayerSlot int8
}

type CommandFlags uint32

const (
	UseOrigin2 CommandFlags = 1 << 0
	UseAngles2 CommandFlags = 1 << 1
	NoInterp   CommandFlags = 1 << 2
)

type CommandInfo [4]CommandInfoSplit

type CommandInfoSplit struct {
	Flags CommandFlags

	ViewOrigin      [3]float32
	ViewAngles      [3]float32
	LocalViewAngles [3]float32

	ViewOrigin2      [3]float32
	ViewAngles2      [3]float32
	LocalViewAngles2 [3]float32
}
