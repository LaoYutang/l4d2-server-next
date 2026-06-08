package protocol

import (
	"bytes"
	"fmt"
	"strings"

	"l4d2-manager-next/pkg/valve/bits"
)

type StringTableEntry struct {
	String   string
	UserData []byte
}

type StringTable struct {
	Name              string
	Strings           []StringTableEntry
	Client            []StringTableEntry
	MaxStrings        int
	EntryBits         int
	IsFilenames       bool
	UserDataFixedSize bool
	UserDataSize      int
	UserDataSizeBits  int
}

func (t *StringTable) ReadFrom(r *bits.Reader) error {
	numStrings, err := r.ReadUint16()
	if err != nil {
		return err
	}

	t.Strings = make([]StringTableEntry, numStrings, t.MaxStrings)

	for i := range t.Strings {
		s, err := r.ReadString()
		if err != nil {
			return err
		}

		t.Strings[i].String = s

		hasUserData, err := r.ReadBool()
		if err != nil {
			return err
		}

		if hasUserData {
			length, err := r.ReadUint16()
			if err != nil {
				return err
			}

			b := make([]byte, length)
			_, err = r.Read(b)
			if err != nil {
				return err
			}

			t.Strings[i].UserData = b
		}
	}

	hasClientSideData, err := r.ReadBool()
	if err != nil {
		return err
	}

	if hasClientSideData {
		numStrings, err = r.ReadUint16()
		if err != nil {
			return err
		}

		t.Client = make([]StringTableEntry, numStrings, t.MaxStrings)

		for i := range t.Client {
			t.Client[i].String, err = r.ReadString()
			if err != nil {
				return err
			}

			hasUserData, err := r.ReadBool()
			if err != nil {
				return err
			}

			if hasUserData {
				size, err := r.ReadUint16()
				if err != nil {
					return err
				}

				t.Client[i].UserData = make([]byte, size)

				_, err = r.Read(t.Client[i].UserData)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (t *StringTable) ReadUpdate(r *bits.Reader, numEntries int) error {
	encodeUsingDictionaries, err := r.ReadBool()
	if err != nil {
		return err
	}

	lastEntry := -1
	lastDictionaryIndex := -1

	history := make([]string, 0, 32)

	for i := 0; i < numEntries; i++ {
		entryIndex := lastEntry + 1

		isNextID, err := r.ReadBool()
		if err != nil {
			return err
		}

		if !isNextID {
			n, err := r.ReadBits(t.EntryBits)
			if err != nil {
				return err
			}

			entryIndex = int(n)
		}

		lastEntry = entryIndex

		var s string
		var u []byte

		haveString, err := r.ReadBool()
		if err != nil {
			return err
		}

		if haveString {
			useDictionary := false
			if encodeUsingDictionaries {
				useDictionary, err = r.ReadBool()
				if err != nil {
					return err
				}
			}

			if useDictionary {
				isSequential, err := r.ReadBool()
				if err != nil {
					return err
				}

				if isSequential {
					lastDictionaryIndex++
				} else {
					panic("TODO: read dictionary index")
				}

				panic("TODO: dictionary")
			} else {
				useSubstring, err := r.ReadBool()
				if err != nil {
					return err
				}

				if useSubstring {
					ssIndex, err := r.ReadBits(5)
					if err != nil {
						return err
					}

					if int(ssIndex) >= len(history) {
						return fmt.Errorf("bogus substring index %d", ssIndex)
					}

					bytesToCopy, err := r.ReadBits(5)
					if err != nil {
						return err
					}

					s, err = r.ReadString()
					if err != nil {
						return err
					}

					historyString := history[ssIndex]

					s = historyString[:bytesToCopy] + s
				} else {
					s, err = r.ReadString()
					if err != nil {
						return err
					}
				}
			}
		}

		haveUserData, err := r.ReadBool()
		if err != nil {
			return err
		}

		if haveUserData {
			readBits := t.UserDataSizeBits
			if t.UserDataFixedSize {
				u = make([]byte, t.UserDataSize)
			} else {
				n, err := r.ReadBits(14)
				if err != nil {
					return err
				}
				u = make([]byte, n)
				readBits = int(n) * 8
			}

			err = r.ReadSlice(u, readBits)
			if err != nil {
				return err
			}
		}

		if entryIndex < len(t.Strings) {
			s = t.Strings[entryIndex].String
			t.Strings[entryIndex].UserData = u
		} else {
			if entryIndex != len(t.Strings) {
				return fmt.Errorf("cannot insert string %q at index %d of %d", s, entryIndex, len(t.Strings))
			}

			t.Strings = append(t.Strings, StringTableEntry{
				String:   s,
				UserData: u,
			})
		}

		if len(history) == 32 {
			copy(history, history[1:])
			history[31] = s
		} else {
			history = append(history, s)
		}
	}

	return nil
}

type StringTables []StringTable

func (t StringTables) Find(name string) *StringTable {
	for i := range t {
		if strings.EqualFold(t[i].Name, name) {
			return &t[i]
		}
	}

	return nil
}

func (t StringTables) ReadFrom(r *bits.Reader) error {
	numTables, err := r.ReadUint8()
	if err != nil {
		return err
	}

	for i := uint8(0); i < numTables; i++ {
		name, err := r.ReadString()
		if err != nil {
			return err
		}

		table := t.Find(name)
		if table == nil {
			return fmt.Errorf("could not find stringtable %q", name)
		}

		err = table.ReadFrom(r)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *StringTables) CreateFromPacket(p *SVC_CreateStringTable) error {
	table := t.Find(p.TableName)
	if table == nil {
		*t = append(*t, StringTable{Name: p.TableName})
		table = &(*t)[len(*t)-1]
	}

	table.MaxStrings = int(p.MaxEntries)
	table.EntryBits = 0
	for x := p.MaxEntries >> 1; x != 0; x >>= 1 {
		table.EntryBits++
	}

	table.IsFilenames = p.IsFilenames
	table.UserDataFixedSize = p.UserDataFixedSize
	table.UserDataSize = int(p.UserDataSizeBytes)
	table.UserDataSizeBits = int(p.UserDataSizeBits)
	table.Strings = make([]StringTableEntry, 0, p.MaxEntries)

	data := p.Data
	if p.DataCompressed {
		var err error

		data, err = decompress(data[8:])
		if err != nil {
			return err
		}
	}

	r := bits.NewReader(bytes.NewReader(data))

	err := table.ReadUpdate(r, int(p.NumEntries))

	return err
}

func (t StringTables) UpdateFromPacket(p *SVC_UpdateStringTable) error {
	if int(p.TableID) > len(t) {
		return fmt.Errorf("out of range string table id %d", p.TableID)
	}

	r := bits.NewReader(bytes.NewReader(p.Data))

	return t[p.TableID].ReadUpdate(r, int(p.ChangedEntries))
}
