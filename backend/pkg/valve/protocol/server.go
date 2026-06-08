package protocol

import (
	"l4d2-manager-next/pkg/valve/bits"
	"l4d2-manager-next/pkg/valve/kvpacker"
)

type SVC_ServerInfo struct {
	Protocol       uint16
	ServerCount    uint32
	IsHLTV         bool
	IsReplay       bool
	IsDedicated    bool
	ClientDLLCRC   uint32
	StringTableCRC uint32
	MaxClasses     uint16
	MapCRC         uint32
	PlayerSlot     uint8
	MaxClients     uint8
	TickInterval   float32
	OS             byte
	GameDir        string
	MapName        string
	SkyName        string
	HostName       string
}

func (p *SVC_ServerInfo) PacketType() int { return svc_ServerInfo }

func (p *SVC_ServerInfo) ReadFrom(r *bits.Reader) error {
	var err error

	p.Protocol, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.ServerCount, err = r.ReadUint32()
	if err != nil {
		return err
	}

	p.IsHLTV, err = r.ReadBool()
	if err != nil {
		return err
	}

	p.IsReplay, err = r.ReadBool()
	if err != nil {
		return err
	}

	p.IsDedicated, err = r.ReadBool()
	if err != nil {
		return err
	}

	p.ClientDLLCRC, err = r.ReadUint32()
	if err != nil {
		return err
	}

	p.StringTableCRC, err = r.ReadUint32()
	if err != nil {
		return err
	}

	p.MaxClasses, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.MapCRC, err = r.ReadUint32()
	if err != nil {
		return err
	}

	p.PlayerSlot, err = r.ReadUint8()
	if err != nil {
		return err
	}

	p.MaxClients, err = r.ReadUint8()
	if err != nil {
		return err
	}

	p.TickInterval, err = r.ReadFloat32()
	if err != nil {
		return err
	}

	p.OS, err = r.ReadUint8()
	if err != nil {
		return err
	}

	p.GameDir, err = r.ReadString()
	if err != nil {
		return err
	}

	p.MapName, err = r.ReadString()
	if err != nil {
		return err
	}

	p.SkyName, err = r.ReadString()
	if err != nil {
		return err
	}

	p.HostName, err = r.ReadString()
	if err != nil {
		return err
	}

	return nil
}

type SVC_SendTable struct {
	NeedsDecoder bool
	BitLength    uint16
	Data         []byte
}

func (p *SVC_SendTable) PacketType() int { return svc_SendTable }

func (p *SVC_SendTable) ReadFrom(r *bits.Reader) error {
	var err error

	p.NeedsDecoder, err = r.ReadBool()
	if err != nil {
		return err
	}

	p.BitLength, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_ClassInfo struct {
	NumClasses     uint16
	CreateOnClient bool
	Info           []ServerClassInfo
}

func (p *SVC_ClassInfo) PacketType() int { return svc_ClassInfo }

func (p *SVC_ClassInfo) ReadFrom(r *bits.Reader) error {
	var err error

	p.NumClasses, err = r.ReadUint16()
	if err != nil {
		return err
	}

	bits := 1
	for x := p.NumClasses >> 1; x != 0; x >>= 1 {
		bits++
	}

	p.CreateOnClient, err = r.ReadBool()
	if err != nil {
		return err
	}

	if !p.CreateOnClient {
		p.Info = nil

		return nil
	}

	p.Info = make([]ServerClassInfo, p.NumClasses)
	for i := range p.Info {
		n, err := r.ReadBits(bits)
		if err != nil {
			return err
		}

		p.Info[i].ClassID = uint32(n)

		p.Info[i].ClassName, err = r.ReadString()
		if err != nil {
			return err
		}

		p.Info[i].DataTableName, err = r.ReadString()
		if err != nil {
			return err
		}
	}

	return nil
}

type SVC_SetPause struct {
	Paused bool
}

func (p *SVC_SetPause) PacketType() int { return svc_SetPause }

func (p *SVC_SetPause) ReadFrom(r *bits.Reader) error {
	var err error

	p.Paused, err = r.ReadBool()

	return err
}

type SVC_CreateStringTable struct {
	TableName  string
	MaxEntries uint16
	// number of bits is smallest number of bits needed to hold MaxEntries+1
	NumEntries        uint16
	BitLength         uint32 // 20 bits
	UserDataFixedSize bool
	UserDataSizeBytes uint16 // 12 bits (only if UserDataFixedSize)
	UserDataSizeBits  uint16 // 4 bits (only if UserDataFixedSize)
	DataCompressed    bool
	IsFilenames       bool
	Data              []byte
}

func (p *SVC_CreateStringTable) PacketType() int { return svc_CreateStringTable }

func (p *SVC_CreateStringTable) ReadFrom(r *bits.Reader) error {
	var err error

	p.TableName, err = r.ReadString()
	if err != nil {
		return err
	}

	p.MaxEntries, err = r.ReadUint16()
	if err != nil {
		return err
	}

	bits := 1
	for x := p.MaxEntries >> 1; x != 0; x >>= 1 {
		bits++
	}

	n, err := r.ReadBits(bits)
	if err != nil {
		return err
	}

	p.NumEntries = uint16(n)

	n, err = r.ReadBits(20)
	if err != nil {
		return err
	}

	p.BitLength = uint32(n)

	p.UserDataFixedSize, err = r.ReadBool()
	if err != nil {
		return err
	}

	if p.UserDataFixedSize {
		n, err = r.ReadBits(12)
		if err != nil {
			return err
		}

		p.UserDataSizeBytes = uint16(n)

		n, err = r.ReadBits(4)
		if err != nil {
			return err
		}

		p.UserDataSizeBits = uint16(n)
	} else {
		p.UserDataSizeBytes = 0
		p.UserDataSizeBits = 0
	}

	p.DataCompressed, err = r.ReadBool()
	if err != nil {
		return err
	}

	p.IsFilenames, err = r.ReadBool()
	if err != nil {
		return err
	}

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_UpdateStringTable struct {
	TableID        uint8  // 5 bits
	ChangedEntries uint16 // if 1, encoded as a single 0 bit, otherwise encoded as a 1 bit followed by a 16 bit integer
	BitLength      uint32 // 20 bits
	Data           []byte
}

func (p *SVC_UpdateStringTable) PacketType() int { return svc_UpdateStringTable }

func (p *SVC_UpdateStringTable) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadBits(5)
	if err != nil {
		return err
	}

	p.TableID = uint8(n)

	b, err := r.ReadBool()
	if err != nil {
		return err
	}

	p.ChangedEntries = 1
	if b {
		p.ChangedEntries, err = r.ReadUint16()
		if err != nil {
			return err
		}
	}

	n, err = r.ReadBits(20)
	if err != nil {
		return err
	}

	p.BitLength = uint32(n)

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_VoiceInit struct {
	Codec   string
	Quality uint8
}

func (p *SVC_VoiceInit) PacketType() int { return svc_VoiceInit }

func (p *SVC_VoiceInit) ReadFrom(r *bits.Reader) error {
	var err error

	p.Codec, err = r.ReadString()
	if err != nil {
		return err
	}

	p.Quality, err = r.ReadUint8()
	if err != nil {
		return err
	}

	return nil
}

type SVC_VoiceData struct {
	FromClient uint8
	Proximity  bool // 8 bits(!)
	BitLength  uint16
	Data       []byte
}

func (p *SVC_VoiceData) PacketType() int { return svc_VoiceData }

func (p *SVC_VoiceData) ReadFrom(r *bits.Reader) error {
	var err error

	p.FromClient, err = r.ReadUint8()
	if err != nil {
		return err
	}

	proximity, err := r.ReadUint8()
	if err != nil {
		return err
	}

	p.Proximity = proximity != 0

	p.BitLength, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_Print struct {
	Text string
}

func (p *SVC_Print) PacketType() int { return svc_Print }

func (p *SVC_Print) ReadFrom(r *bits.Reader) error {
	var err error

	p.Text, err = r.ReadString()

	return err
}

type SVC_Sounds struct {
	Reliable  bool
	NumSounds uint8  // if Reliable, there is 1 sound
	BitLength uint16 // if Reliable, sent as 8 bits
	Data      []byte
}

func (p *SVC_Sounds) PacketType() int { return svc_Sounds }

func (p *SVC_Sounds) ReadFrom(r *bits.Reader) error {
	var err error

	p.Reliable, err = r.ReadBool()
	if err != nil {
		return err
	}

	if p.Reliable {
		p.NumSounds = 1
		n, err := r.ReadUint8()
		if err != nil {
			return err
		}

		p.BitLength = uint16(n)
	} else {
		p.NumSounds, err = r.ReadUint8()
		if err != nil {
			return err
		}

		p.BitLength, err = r.ReadUint16()
		if err != nil {
			return err
		}
	}

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_SetView struct {
	EntityIndex uint16 // 11 bits
}

func (p *SVC_SetView) PacketType() int { return svc_SetView }

func (p *SVC_SetView) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadBits(11)
	if err != nil {
		return err
	}

	p.EntityIndex = uint16(n)

	return nil
}

type SVC_FixAngle struct {
	Relative bool
	// fixed point angle (0 to 65536 -> 0 to 360 degrees)
	Pitch, Yaw, Roll uint16
}

func (p *SVC_FixAngle) PacketType() int { return svc_FixAngle }

func (p *SVC_FixAngle) ReadFrom(r *bits.Reader) error {
	var err error

	p.Relative, err = r.ReadBool()
	if err != nil {
		return err
	}

	p.Pitch, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.Yaw, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.Roll, err = r.ReadUint16()
	if err != nil {
		return err
	}

	return nil
}

type SVC_CrosshairAngle struct {
	// fixed point angle (0 to 65536 -> 0 to 360 degrees)
	Pitch, Yaw, Roll uint16
}

func (p *SVC_CrosshairAngle) PacketType() int { return svc_CrosshairAngle }

func (p *SVC_CrosshairAngle) ReadFrom(r *bits.Reader) error {
	var err error

	p.Pitch, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.Yaw, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.Roll, err = r.ReadUint16()
	if err != nil {
		return err
	}

	return nil
}

type SVC_BSPDecal struct {
	Position [3]float32 // BitVec3Coord

	DecalTextureIndex uint16 // 9 bits

	// if these are both 0, write a 0 bit and skip them.
	// otherwise, write a 1 bit and then the next two fields.
	EntityIndex uint16 // 11 bits
	ModelIndex  uint16 // 11 bits

	LowPriority bool
}

func (p *SVC_BSPDecal) PacketType() int { return svc_BSPDecal }

func (p *SVC_BSPDecal) ReadFrom(r *bits.Reader) error {
	var err error

	p.Position, err = r.ReadBitVec3Coord(14, 5)
	if err != nil {
		return err
	}

	n, err := r.ReadBits(9)
	if err != nil {
		return err
	}

	p.DecalTextureIndex = uint16(n)

	b, err := r.ReadBool()
	if err != nil {
		return err
	}

	if b {
		n, err = r.ReadBits(11)
		if err != nil {
			return err
		}

		p.EntityIndex = uint16(n)

		n, err = r.ReadBits(11)
		if err != nil {
			return err
		}

		p.ModelIndex = uint16(n)
	} else {
		p.EntityIndex = 0
		p.ModelIndex = 0
	}

	p.LowPriority, err = r.ReadBool()
	if err != nil {
		return err
	}

	return nil
}

type SVC_SplitScreen struct {
	Remove      bool
	Slot        uint8  // 2 bits
	EntityIndex uint16 // 11 bits (name is a guess based on size; haven't verified yet)
}

func (p *SVC_SplitScreen) PacketType() int { return svc_SplitScreen }

func (p *SVC_SplitScreen) ReadFrom(r *bits.Reader) error {
	var err error

	p.Remove, err = r.ReadBool()
	if err != nil {
		return err
	}

	n, err := r.ReadBits(2)
	if err != nil {
		return err
	}

	p.Slot = uint8(n)

	n, err = r.ReadBits(11)
	if err != nil {
		return err
	}

	p.EntityIndex = uint16(n)

	return nil
}

type SVC_UserMessage struct {
	MessageType uint8
	BitLength   uint16 // 12 bits
	Data        []byte
}

func (p *SVC_UserMessage) PacketType() int { return svc_UserMessage }

func (p *SVC_UserMessage) ReadFrom(r *bits.Reader) error {
	var err error

	p.MessageType, err = r.ReadUint8()
	if err != nil {
		return err
	}

	n, err := r.ReadBits(12)
	if err != nil {
		return err
	}

	p.BitLength = uint16(n)

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_EntityMessage struct {
	EntityIndex uint16 // 11 bits
	ClassID     uint16 // 9 bits
	BitLength   uint16 // 11 bits
	Data        []byte
}

func (p *SVC_EntityMessage) PacketType() int { return svc_EntityMessage }

func (p *SVC_EntityMessage) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadBits(11)
	if err != nil {
		return err
	}

	p.EntityIndex = uint16(n)

	n, err = r.ReadBits(9)
	if err != nil {
		return err
	}

	p.ClassID = uint16(n)

	n, err = r.ReadBits(11)
	if err != nil {
		return err
	}

	p.BitLength = uint16(n)

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_GameEvent struct {
	BitLength uint16 // 13 bits
	Data      []byte
}

func (p *SVC_GameEvent) PacketType() int { return svc_GameEvent }

func (p *SVC_GameEvent) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadBits(13)
	if err != nil {
		return err
	}

	p.BitLength = uint16(n)

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_PacketEntities struct {
	MaxEntries     uint16 // 11 bits
	DeltaFrom      int32  // if -1, written as a single 0 bit. otherwise, prefixed with a 1 bit.
	Baseline       bool
	UpdatedEntries uint16 // 11 bits
	BitLength      uint32 // 20 bits
	UpdateBaseline bool
	Data           []byte
}

func (p *SVC_PacketEntities) PacketType() int { return svc_PacketEntities }

func (p *SVC_PacketEntities) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadBits(11)
	if err != nil {
		return err
	}

	p.MaxEntries = uint16(n)

	b, err := r.ReadBool()
	if err != nil {
		return err
	}

	if b {
		p.DeltaFrom, err = r.ReadInt32()
		if err != nil {
			return err
		}
	} else {
		p.DeltaFrom = -1
	}

	p.Baseline, err = r.ReadBool()
	if err != nil {
		return err
	}

	n, err = r.ReadBits(11)
	if err != nil {
		return err
	}

	p.UpdatedEntries = uint16(n)

	n, err = r.ReadBits(20)
	if err != nil {
		return err
	}

	p.BitLength = uint32(n)

	p.UpdateBaseline, err = r.ReadBool()

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_TempEntities struct {
	NumEntries uint8
	BitLength  uint32 // 17 bits
	Data       []byte
}

func (p *SVC_TempEntities) PacketType() int { return svc_TempEntities }

func (p *SVC_TempEntities) ReadFrom(r *bits.Reader) error {
	var err error

	p.NumEntries, err = r.ReadUint8()
	if err != nil {
		return err
	}

	n, err := r.ReadBits(17)
	if err != nil {
		return err
	}

	p.BitLength = uint32(n)

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_Prefetch struct {
	SoundIndex uint16 // 13 bits
}

func (p *SVC_Prefetch) PacketType() int { return svc_Prefetch }

func (p *SVC_Prefetch) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadBits(13)
	if err != nil {
		return err
	}

	p.SoundIndex = uint16(n)

	return nil
}

type SVC_Menu struct {
	Type uint16
	Data []byte // 16-bit length but can only be up to 4096 bytes
}

func (p *SVC_Menu) PacketType() int { return svc_Menu }

func (p *SVC_Menu) ReadFrom(r *bits.Reader) error {
	var err error

	p.Type, err = r.ReadUint16()
	if err != nil {
		return err
	}

	length, err := r.ReadUint16()
	if err != nil {
		return err
	}

	p.Data = make([]byte, length)
	_, err = r.Read(p.Data)

	return err
}

type SVC_GameEventList struct {
	NumEvents uint16 // 9 bits
	BitLength uint32 // 20 bits
	Data      []byte
}

func (p *SVC_GameEventList) PacketType() int { return svc_GameEventList }

func (p *SVC_GameEventList) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadBits(9)
	if err != nil {
		return err
	}

	p.NumEvents = uint16(n)

	n, err = r.ReadBits(20)
	if err != nil {
		return err
	}

	p.BitLength = uint32(n)

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type SVC_GetCvarValue struct {
	Cookie int32
	Name   string
}

func (p *SVC_GetCvarValue) PacketType() int { return svc_GetCvarValue }

func (p *SVC_GetCvarValue) ReadFrom(r *bits.Reader) error {
	var err error

	p.Cookie, err = r.ReadInt32()
	if err != nil {
		return err
	}

	p.Name, err = r.ReadString()
	if err != nil {
		return err
	}

	return nil
}

type SVC_CmdKeyValues struct {
	Data kvpacker.Data // 32-bit length
}

func (p *SVC_CmdKeyValues) PacketType() int { return svc_CmdKeyValues }

func (p *SVC_CmdKeyValues) ReadFrom(r *bits.Reader) error {
	length, err := r.ReadUint32()

	b := make([]byte, length)
	_, err = r.Read(b)
	if err != nil {
		return err
	}

	p.Data = nil
	return p.Data.UnmarshalBinary(b)
}
