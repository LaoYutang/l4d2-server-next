//go:generate go run golang.org/x/tools/cmd/stringer -type SignonState -trimprefix SignonState

package protocol

import "l4d2-manager-next/pkg/valve/bits"

type NET_NOP struct {
	// no data
}

func (p *NET_NOP) PacketType() int { return net_NOP }

func (p *NET_NOP) ReadFrom(r *bits.Reader) error {
	// no data

	return nil
}

type NET_Disconnect struct {
	Reason string
}

func (p *NET_Disconnect) PacketType() int { return net_Disconnect }

func (p *NET_Disconnect) ReadFrom(r *bits.Reader) error {
	var err error

	p.Reason, err = r.ReadString()

	return err
}

type NET_File struct {
	TransferID uint32
	Name       string
	IsRequest  bool // if it's not a request, this is a rejection
}

func (p *NET_File) PacketType() int { return net_File }

func (p *NET_File) ReadFrom(r *bits.Reader) error {
	var err error

	p.TransferID, err = r.ReadUint32()
	if err != nil {
		return err
	}

	p.Name, err = r.ReadString()
	if err != nil {
		return err
	}

	p.IsRequest, err = r.ReadBool()
	if err != nil {
		return err
	}

	return nil
}

type NET_SplitScreenUser struct {
	Slot uint8 // 2 bits
}

func (p *NET_SplitScreenUser) PacketType() int { return net_SplitScreenUser }

func (p *NET_SplitScreenUser) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadBits(2)
	if err != nil {
		return err
	}

	p.Slot = uint8(n)

	return nil
}

type NET_Tick struct {
	TickNumber int32
	// 0.5 fixed point seconds (that is, max value is 655.35 milliseconds)
	FrameTime       uint16
	FrameTimeStdDev uint16
}

func (p *NET_Tick) PacketType() int { return net_Tick }

func (p *NET_Tick) ReadFrom(r *bits.Reader) error {
	var err error

	p.TickNumber, err = r.ReadInt32()
	if err != nil {
		return err
	}

	p.FrameTime, err = r.ReadUint16()
	if err != nil {
		return err
	}

	p.FrameTimeStdDev, err = r.ReadUint16()
	if err != nil {
		return err
	}

	return nil
}

type NET_StringCmd struct {
	Command string
}

func (p *NET_StringCmd) PacketType() int { return net_StringCmd }

func (p *NET_StringCmd) ReadFrom(r *bits.Reader) error {
	var err error

	p.Command, err = r.ReadString()

	return err
}

type NET_SetConVar struct {
	// uint8 count followed by strings
	ConVars []KeyValuePair
}

func (p *NET_SetConVar) PacketType() int { return net_SetConVar }

func (p *NET_SetConVar) ReadFrom(r *bits.Reader) error {
	count, err := r.ReadUint8()
	if err != nil {
		return err
	}

	p.ConVars = make([]KeyValuePair, count)
	for i := range p.ConVars {
		p.ConVars[i].Key, err = r.ReadString()
		if err != nil {
			return err
		}

		p.ConVars[i].Value, err = r.ReadString()
		if err != nil {
			return err
		}
	}

	return nil
}

type SignonState uint8

const (
	SignonStateNone        SignonState = 0
	SignonStateChallenge   SignonState = 1
	SignonStateConnected   SignonState = 2
	SignonStateNew         SignonState = 3
	SignonStatePreSpawn    SignonState = 4
	SignonStateSpawn       SignonState = 5
	SignonStateFull        SignonState = 6
	SignonStateChangeLevel SignonState = 7
)

type NET_SignonState struct {
	State      SignonState
	SpawnCount uint32
}

func (p *NET_SignonState) PacketType() int { return net_SignonState }

func (p *NET_SignonState) ReadFrom(r *bits.Reader) error {
	n, err := r.ReadUint8()
	if err != nil {
		return err
	}

	p.State = SignonState(n)

	p.SpawnCount, err = r.ReadUint32()
	if err != nil {
		return err
	}

	return nil
}
