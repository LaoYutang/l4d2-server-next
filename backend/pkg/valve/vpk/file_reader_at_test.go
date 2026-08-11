package vpk

import (
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenReaderAtReadsAcrossMetadataAndArchiveData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reader-at.vpk")
	metadata := []byte("pre")
	body := []byte("body")
	all := append(append([]byte(nil), metadata...), body...)

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("create VPK: %v", err)
	}
	archive := &Archive{
		Header: Header{Magic: Magic, Version: 1},
		Files: []File{{
			Dir:      "scripts",
			Base:     "test",
			Ext:      "nut",
			Metadata: metadata,
			DirEntry: DirEntry{
				CRC:           crc32.ChecksumIEEE(all),
				MetadataBytes: uint16(len(metadata)),
				DataLocation: []DataChunk{{
					ArchiveIndex: selfArchive,
					EntryLength:  uint32(len(body)),
				}},
			},
		}},
	}
	if err := WriteDirectory(out, archive); err != nil {
		out.Close()
		t.Fatalf("write VPK directory: %v", err)
	}
	if _, err := out.Write(body); err != nil {
		out.Close()
		t.Fatalf("write VPK body: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close VPK: %v", err)
	}

	opener := Single(path)
	defer opener.Close()
	parsed, err := opener.ReadArchive()
	if err != nil {
		t.Fatalf("read VPK: %v", err)
	}
	if len(parsed.Files) != 1 {
		t.Fatalf("file count = %d, want 1", len(parsed.Files))
	}

	reader := parsed.Files[0].OpenReaderAt(opener)
	buffer := make([]byte, 5)
	n, err := reader.ReadAt(buffer, 1)
	if err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}
	if n != len(buffer) || string(buffer) != "rebod" {
		t.Fatalf("ReadAt() = %d %q, want 5 %q", n, buffer, "rebod")
	}

	tail := make([]byte, 4)
	n, err = reader.ReadAt(tail, int64(len(all)-2))
	if n != 2 || err != io.EOF || string(tail[:n]) != "dy" {
		t.Fatalf("tail ReadAt() = %d %q %v, want 2 %q EOF", n, tail[:n], err, "dy")
	}
}
