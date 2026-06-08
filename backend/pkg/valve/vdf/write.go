package vdf

import "io"

var _ io.WriterTo = (*KeyValues)(nil)

func (kv *KeyValues) WriteTo(w io.Writer) (int64, error) {
	var n int64

	err := kv.write(w, 0, &n)

	return n, err
}

func (kv *KeyValues) write(w io.Writer, indent int, total *int64) error {
	var err error

	write := func(s string) {
		if err == nil {
			var n int

			n, err = io.WriteString(w, s)

			*total += int64(n)
		}
	}

	doIndent := func() {
		for i := 0; i < indent; i++ {
			write("\t")
		}
	}

	doIndent()

	write("\"")
	write(Escape.Replace(kv.Key))

	if kv.HasValue {
		write("\"\t\t\"")
		write(Escape.Replace(kv.Value))
		write("\"")
	} else {
		write("\"\n")
		doIndent()
		write("{\n")

		if kv.sub != nil && err == nil {
			err = kv.sub.write(w, indent+1, total)
		}

		doIndent()
		write("}")
	}

	if kv.Cond != "" {
		write(" ")
		write(kv.Cond)
	}

	write("\n")

	if kv.peer != nil && err == nil {
		err = kv.peer.write(w, indent, total)
	}

	return err
}
