package vpk

import (
	"fmt"
	"io"
	"os"
)

// Single returns a new Opener with a single-file VPK.
func Single(path string) *Opener {
	return &Opener{
		prefix: path,
		single: true,
	}
}

// Dir returns a new Opener with a given prefix.
func Dir(prefix string) *Opener {
	return &Opener{
		prefix: prefix,
	}
}

// Opener caches file handles for a VPK.
type Opener struct {
	prefix string
	dir    *os.File
	data   []*os.File
	single bool
}

// Dir opens and returns prefix_dir.vpk.
func (o *Opener) Dir() (*os.File, error) {
	if o.dir != nil {
		return o.dir, nil
	}

	name := o.prefix
	if !o.single {
		name += "_dir.vpk"
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	o.dir = f

	return f, nil
}

// ReadArchive reads the VPK archive tree for this Opener.
func (o *Opener) ReadArchive() (*Archive, error) {
	f, err := o.Dir()
	if err != nil {
		return nil, err
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	return ReadArchive(f)
}

// Archive opens and returns prefix_###.vpk.
func (o *Opener) Archive(index int) (*os.File, error) {
	if index == selfArchive {
		return o.Dir()
	}

	if o.single {
		return nil, fmt.Errorf("vpk: cannot request archive volume %d of a single archive", index)
	}

	if len(o.data) > index && o.data[index] != nil {
		return o.data[index], nil
	}

	f, err := os.Open(fmt.Sprintf("%s_%03d.vpk", o.prefix, index))
	if err != nil {
		return nil, err
	}

	for len(o.data) <= index {
		o.data = append(o.data, nil)
	}

	o.data[index] = f

	return f, nil
}

// Close closes all files that the Opener has opened, and returns the first
// error encountered.
func (o *Opener) Close() error {
	var firstError error

	if o.dir != nil {
		if err := o.dir.Close(); firstError == nil {
			firstError = err
		}

		o.dir = nil
	}

	for _, f := range o.data {
		if f != nil {
			if err := f.Close(); firstError == nil {
				firstError = err
			}
		}
	}

	o.data = nil

	return firstError
}
