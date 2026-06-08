package vdf

import "strings"

func (kv *KeyValues) FindKey(name string) *KeyValues {
	parts := strings.Split(name, "/")

	for len(parts) > 1 {
		kv = kv.FindKey(parts[0])

		parts = parts[1:]
	}

	if kv == nil {
		return nil
	}

	for c := kv.sub; c != nil; c = c.peer {
		if strings.EqualFold(c.Key, parts[0]) {
			return c
		}
	}

	return nil
}

func (kv *KeyValues) FirstSubKey() *KeyValues {
	if kv == nil {
		return nil
	}

	return kv.sub
}

func (kv *KeyValues) NextSubKey() *KeyValues {
	if kv == nil {
		return nil
	}

	return kv.peer
}

func (kv *KeyValues) FirstValue() *KeyValues {
	if kv == nil {
		return nil
	}

	if kv.sub == nil {
		return nil
	}

	if kv.sub.sub == nil {
		return kv.sub
	}

	return kv.sub.NextValue()
}

func (kv *KeyValues) NextValue() *KeyValues {
	if kv == nil {
		return nil
	}

	next := kv.peer

	for next != nil && next.sub != nil {
		next = next.peer
	}

	return next
}

func (kv *KeyValues) FirstTrueSubKey() *KeyValues {
	if kv == nil {
		return nil
	}

	if kv.sub == nil {
		return nil
	}

	if kv.sub.sub != nil {
		return kv.sub
	}

	return kv.sub.NextTrueSubKey()
}

func (kv *KeyValues) NextTrueSubKey() *KeyValues {
	if kv == nil {
		return nil
	}

	next := kv.peer

	for next != nil && next.sub == nil {
		next = next.peer
	}

	return next
}
