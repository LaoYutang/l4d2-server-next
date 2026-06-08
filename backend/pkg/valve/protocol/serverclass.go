package protocol

import "l4d2-manager-next/pkg/valve/bits"

type ServerClassInfo struct {
	ClassID       uint32
	ClassName     string
	DataTableName string
}

type ServerClasses []ServerClassInfo

func (c *ServerClasses) ReadFrom(r *bits.Reader) error {
	count, err := r.ReadUint16()
	if err != nil {
		return err
	}

	*c = make(ServerClasses, count)

	for i := range *c {
		classID, err := r.ReadUint16()
		if err != nil {
			return err
		}

		(*c)[i].ClassID = uint32(classID)

		(*c)[i].ClassName, err = r.ReadString()
		if err != nil {
			return err
		}

		(*c)[i].DataTableName, err = r.ReadString()
		if err != nil {
			return err
		}
	}

	return nil
}
