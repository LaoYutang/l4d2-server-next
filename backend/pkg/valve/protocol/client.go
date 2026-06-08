package protocol

import (
	"fmt"

	"l4d2-manager-next/pkg/valve/bits"
	"l4d2-manager-next/pkg/valve/kvpacker"
)

type CLC_ClientInfo struct {
	ServerCount   uint32
	SendTableCRC  uint32
	IsHLTV        bool
	AccountID     uint32
	Name          string
	CustomFileCRC [4]*uint32
}

func (p *CLC_ClientInfo) PacketType() int { return clc_ClientInfo }

func (p *CLC_ClientInfo) ReadFrom(r *bits.Reader) error {
	var err error

	p.ServerCount, err = r.ReadUint32()
	if err != nil {
		return err
	}

	p.SendTableCRC, err = r.ReadUint32()
	if err != nil {
		return err
	}

	p.IsHLTV, err = r.ReadBool()
	if err != nil {
		return err
	}

	p.AccountID, err = r.ReadUint32()
	if err != nil {
		return err
	}

	p.Name, err = r.ReadString()
	if err != nil {
		return err
	}

	for i := range p.CustomFileCRC {
		haveFile, err := r.ReadBool()
		if err != nil {
			return err
		}

		if !haveFile {
			p.CustomFileCRC[i] = nil

			continue
		}

		p.CustomFileCRC[i] = new(uint32)
		*p.CustomFileCRC[i], err = r.ReadUint32()
		if err != nil {
			return err
		}
	}

	return nil
}

type CLC_Move struct {
	NewCommands    uint8 // 4 bits
	BackupCommands uint8 // 3 bits
	BitLength      uint16
	UserCmdData    []byte
}

func (p *CLC_Move) PacketType() int { return clc_Move }

func (p *CLC_Move) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadBits(4)
	if err != nil {
		return err
	}

	p.NewCommands = uint8(n)

	n, err = r.ReadBits(3)
	if err != nil {
		return err
	}

	p.BackupCommands = uint8(n)

	p.BitLength, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.UserCmdData = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.UserCmdData, int(p.BitLength))

	return err
}

type CLC_VoiceData struct {
	BitLength uint16
	Data      []byte
}

func (p *CLC_VoiceData) PacketType() int { return clc_VoiceData }

func (p *CLC_VoiceData) ReadFrom(r *bits.Reader) error {
	var err error

	p.BitLength, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.Data = make([]byte, (p.BitLength+7)/8)
	err = r.ReadSlice(p.Data, int(p.BitLength))

	return err
}

type CLC_BaselineAck struct {
	Tick     uint32
	Baseline bool // alternates every time the baseline changes
}

func (p *CLC_BaselineAck) PacketType() int { return clc_BaselineAck }

func (p *CLC_BaselineAck) ReadFrom(r *bits.Reader) error {
	var err error

	p.Tick, err = r.ReadUint32()
	if err != nil {
		return err
	}

	p.Baseline, err = r.ReadBool()
	if err != nil {
		return err
	}

	return nil
}

type CLC_ListenEvents struct {
	Flags [(1 << 9) / 32]uint32
}

func (p *CLC_ListenEvents) PacketType() int { return clc_ListenEvents }

func (p *CLC_ListenEvents) ReadFrom(r *bits.Reader) error {
	var err error

	for i := range p.Flags {
		p.Flags[i], err = r.ReadUint32()
		if err != nil {
			return err
		}
	}

	return nil
}

type CLC_RespondCvarValue struct {
	Cookie     int32
	StatusCode int8 // 4 bits
	CVar       KeyValuePair
}

func (p *CLC_RespondCvarValue) PacketType() int { return clc_RespondCvarValue }

func (p *CLC_RespondCvarValue) ReadFrom(r *bits.Reader) error {
	var err error

	p.Cookie, err = r.ReadInt32()
	if err != nil {
		return err
	}

	n, err := r.ReadSigned(4)
	if err != nil {
		return err
	}

	p.StatusCode = int8(n)

	p.CVar.Key, err = r.ReadString()
	if err != nil {
		return err
	}

	p.CVar.Value, err = r.ReadString()
	if err != nil {
		return err
	}

	return nil
}

var commonPaths = [...]string{
	"GAME",
	"MOD",
}

var commonFilenamePrefixes = [...]string{
	"materials",
	"models",
	"sounds",
	"scripts",
}

type CLC_FileCRCCheck struct {
	// reserved bit (0)
	// 2 bits (0 or 1+index into commonPaths)
	// if 0, string
	Path string
	// 3 bits (0 or 1+index into commonFilenamePrefixes)
	// if non-zero, implied slash
	// string
	FileName string
	CRC32    uint32
}

func (p *CLC_FileCRCCheck) PacketType() int { return clc_FileCRCCheck }

func (p *CLC_FileCRCCheck) ReadFrom(r *bits.Reader) error {
	reserved, err := r.ReadBool()
	if err != nil {
		return err
	}

	if reserved {
		return fmt.Errorf("reserved bit is set to wrong value")
	}

	i, err := r.ReadBits(2)
	if err != nil {
		return err
	}

	if int(i) > len(commonPaths) {
		return fmt.Errorf("out-of-range path index %d > %d", i, len(commonPaths))
	}

	if i == 0 {
		p.Path, err = r.ReadString()
		if err != nil {
			return err
		}
	} else {
		p.Path = commonPaths[i-1]
	}

	i, err = r.ReadBits(3)
	if err != nil {
		return err
	}

	if int(i) > len(commonFilenamePrefixes) {
		return fmt.Errorf("out-of-range prefix index %d > %d", i, len(commonFilenamePrefixes))
	}

	prefix := ""
	if i != 0 {
		prefix = commonFilenamePrefixes[i-1] + "/"
	}

	s, err := r.ReadString()
	if err != nil {
		return err
	}

	p.FileName = prefix + s

	p.CRC32, err = r.ReadUint32()
	if err != nil {
		return err
	}

	return nil
}

type CLC_LoadingProgress struct {
	Progress uint8
}

func (p *CLC_LoadingProgress) PacketType() int { return clc_LoadingProgress }

func (p *CLC_LoadingProgress) ReadFrom(r *bits.Reader) error {
	var err error

	p.Progress, err = r.ReadUint8()

	return err
}

type CLC_SplitPlayerConnect struct {
	ConVars NET_SetConVar
}

func (p *CLC_SplitPlayerConnect) PacketType() int { return clc_SplitPlayerConnect }

func (p *CLC_SplitPlayerConnect) ReadFrom(r *bits.Reader) error {
	return p.ConVars.ReadFrom(r)
}

type CLC_CmdKeyValues struct {
	Data kvpacker.Data // 32-bit length
}

func (p *CLC_CmdKeyValues) PacketType() int { return clc_CmdKeyValues }

func (p *CLC_CmdKeyValues) ReadFrom(r *bits.Reader) error {
	length, err := r.ReadUint32()

	b := make([]byte, length)
	_, err = r.Read(b)
	if err != nil {
		return err
	}

	p.Data = nil
	return p.Data.UnmarshalBinary(b)
}
