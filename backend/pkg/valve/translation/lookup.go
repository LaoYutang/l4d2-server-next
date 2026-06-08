package translation

import (
	"hash/crc32"
	"strings"

	"l4d2-manager-next/pkg/valve/vccd"
	"l4d2-manager-next/pkg/valve/vdf"
)

type Lookup interface {
	Find(key string) (string, bool)
	FindSafe(key string) string
}

func normalizeKey(key string) string {
	key = strings.ToLower(key)
	key = strings.TrimPrefix(key, "#")

	return key
}

func findSafe(s string, ok bool) string {
	if ok {
		return s
	}

	return "#FIXME_LOCALIZATION_FAIL_MISSING_STRING"
}

type combinedLookup struct {
	l []Lookup
}

func (l *combinedLookup) Find(key string) (string, bool) {
	// go through list backwards to simulate overwriting
	for i := len(l.l) - 1; i >= 0; i-- {
		s, ok := l.l[i].Find(key)
		if ok {
			return s, true
		}
	}

	return "", false
}

func (l *combinedLookup) FindSafe(key string) string {
	return findSafe(l.Find(key))
}

func Combine(l ...Lookup) Lookup {
	return &combinedLookup{l}
}

type stringLookup struct {
	m map[string]string
}

func (l *stringLookup) Find(key string) (string, bool) {
	s, ok := l.m[normalizeKey(key)]

	return s, ok
}

func (l *stringLookup) FindSafe(key string) string {
	return findSafe(l.Find(key))
}

func FromKeyValues(kv *vdf.KeyValues) Lookup {
	m := make(map[string]string)

	for c := kv.FindKey("Tokens").FirstValue(); c != nil; c = c.NextValue() {
		key := normalizeKey(c.Key)
		if strings.HasPrefix(key, "[english]") {
			continue
		}

		m[key] = c.Value
	}

	return &stringLookup{m}
}

type captionLookup struct {
	d *vccd.CaptionDictionary
}

func (l *captionLookup) Find(key string) (string, bool) {
	key = normalizeKey(key)

	i := l.d.Lookup.FindHash(crc32.ChecksumIEEE([]byte(key)))
	if i < 0 {
		return "", false
	}

	return l.d.Lookup[i].String(l.d), true
}

func (l *captionLookup) FindSafe(key string) string {
	return findSafe(l.Find(key))
}

func FromCaptionDictionary(d *vccd.CaptionDictionary) Lookup {
	return &captionLookup{d}
}
