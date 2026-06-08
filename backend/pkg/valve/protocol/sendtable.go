//go:generate go run golang.org/x/tools/cmd/stringer -type SendPropType -trimprefix SendProp

package protocol

import (
	"l4d2-manager-next/pkg/valve/bits"
)

type SendPropType uint8

const (
	SendPropInt       SendPropType = 0
	SendPropFloat     SendPropType = 1
	SendPropVector    SendPropType = 2
	SendPropVectorXY  SendPropType = 3
	SendPropString    SendPropType = 4
	SendPropArray     SendPropType = 5
	SendPropDataTable SendPropType = 6
	SendPropInt64     SendPropType = 7
	NumSendPropTypes  SendPropType = 8
)

type SendPropFlags uint32

const (
	SPropUnsigned              SendPropFlags = 1 << 0
	SPropCoord                 SendPropFlags = 1 << 1
	SPropNoScale               SendPropFlags = 1 << 2
	SPropRoundDown             SendPropFlags = 1 << 3
	SPropRoundUp               SendPropFlags = 1 << 4
	SPropNormal                SendPropFlags = 1 << 5
	SPropExclude               SendPropFlags = 1 << 6
	SPropXYZE                  SendPropFlags = 1 << 7
	SPropInsideArray           SendPropFlags = 1 << 8
	SPropProxyAlwaysYes        SendPropFlags = 1 << 9
	SPropIsAVectorElem         SendPropFlags = 1 << 10
	SPropCollapsible           SendPropFlags = 1 << 11
	SPropCoordMP               SendPropFlags = 1 << 12
	SPropCoordMPLowPrecision   SendPropFlags = 1 << 13
	SPropCoordMPIntegral       SendPropFlags = 1 << 14
	SPropCellCoord             SendPropFlags = 1 << 15
	SPropCellCoordLowPrecision SendPropFlags = 1 << 16
	SPropCellCoordIntegral     SendPropFlags = 1 << 17
	SPropChangesOften          SendPropFlags = 1 << 18
)

type SendProp struct {
	Name          string
	ExcludeDTName string
	Type          SendPropType
	Priority      uint8
	Flags         SendPropFlags
	NumElements   uint16
	LowValue      float32
	HighValue     float32
	Bits          uint8
}

type SendTable struct {
	NeedsDecoder bool
	Name         string
	Props        []SendProp
}

func (t *SendTable) ReadFrom(r *bits.Reader) error {
	var err error

	t.Name, err = r.ReadString()
	if err != nil {
		return err
	}

	numProps, err := r.ReadBits(10)
	if err != nil {
		return err
	}

	t.Props = make([]SendProp, numProps)
	for i := range t.Props {
		n, err := r.ReadBits(5)
		if err != nil {
			return err
		}

		t.Props[i].Type = SendPropType(n)

		t.Props[i].Name, err = r.ReadString()
		if err != nil {
			return err
		}

		n, err = r.ReadBits(19)
		if err != nil {
			return err
		}

		t.Props[i].Flags = SendPropFlags(n)

		t.Props[i].Priority, err = r.ReadUint8()
		if err != nil {
			return err
		}

		if t.Props[i].Type == SendPropDataTable || t.Props[i].Flags&SPropExclude != 0 {
			t.Props[i].ExcludeDTName, err = r.ReadString()
			if err != nil {
				return err
			}
		} else if t.Props[i].Type == SendPropArray {
			n, err = r.ReadBits(10)
			if err != nil {
				return err
			}

			t.Props[i].NumElements = uint16(n)
		} else {
			t.Props[i].LowValue, err = r.ReadBitFloat32()
			if err != nil {
				return err
			}

			t.Props[i].LowValue, err = r.ReadBitFloat32()
			if err != nil {
				return err
			}

			n, err = r.ReadBits(7)
			if err != nil {
				return err
			}

			t.Props[i].Bits = uint8(n)
		}
	}

	return nil
}

type SendTables []SendTable

func (t *SendTables) ReadFrom(r *bits.Reader) error {
	*t = nil

	for {
		ok, err := r.ReadBool()
		if err != nil {
			return err
		}

		if !ok {
			return nil
		}

		needsDecoder, err := r.ReadBool()
		if err != nil {
			return err
		}

		*t = append(*t, SendTable{NeedsDecoder: needsDecoder})

		err = (*t)[len(*t)-1].ReadFrom(r)
		if err != nil {
			return err
		}
	}
}
