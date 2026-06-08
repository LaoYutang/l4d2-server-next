package vpk

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

func WriteDirectory(w io.Writer, a *Archive) error {
	if a.Version != 1 {
		return fmt.Errorf("vpk: this library cannot yet write version %d files", a.Version)
	}

	header := a.Header

	var tree bytes.Buffer
	var lastExt, lastDir string

	for _, f := range a.Files {
		if f.Ext != lastExt {
			if lastExt != "" {
				tree.WriteByte(0)
				tree.WriteByte(0)
			}
			tree.WriteString(f.Ext)
			tree.WriteByte(0)
			tree.WriteString(f.Dir)
			tree.WriteByte(0)
			lastExt = f.Ext
			lastDir = f.Dir
		} else if f.Dir != lastDir {
			tree.WriteByte(0)
			tree.WriteString(f.Dir)
			tree.WriteByte(0)
			lastDir = f.Dir
		}

		tree.WriteString(f.Base)
		tree.WriteByte(0)

		err := binary.Write(&tree, binary.LittleEndian, &f.DirEntry.CRC)
		if err != nil {
			// should never happen as we are writing to memory
			panic(err)
		}
		err = binary.Write(&tree, binary.LittleEndian, &f.DirEntry.MetadataBytes)
		if err != nil {
			panic(err)
		}
		err = binary.Write(&tree, binary.LittleEndian, &f.DirEntry.DataLocation)
		if err != nil {
			panic(err)
		}
		err = binary.Write(&tree, binary.LittleEndian, uint16(terminator))
		if err != nil {
			panic(err)
		}

		tree.Write(f.Metadata)
	}

	if lastExt != "" {
		tree.WriteByte(0)
		tree.WriteByte(0)
		tree.WriteByte(0)
	}

	header.TreeSize = uint32(tree.Len())

	err := binary.Write(w, binary.LittleEndian, &header)
	if err != nil {
		return err
	}

	_, err = tree.WriteTo(w)
	if err != nil {
		return err
	}

	return nil
}
