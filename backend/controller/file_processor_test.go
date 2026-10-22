package controller

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"l4d2-manager-next/consts"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/pkg/valve/vpk"

	"github.com/nwaples/rardecode/v2"
)

func TestProcessArchiveExtractsOnlyVPKs(t *testing.T) {
	for _, name := range []string{"rar4-stored.rar", "rar5-stored.rar", "rar4-normal.rar", "rar5-normal.rar", "rar4-solid.rar", "rar5-solid.rar", "rar5-modern-normal.rar", "rar5-modern-solid.rar", "normal.7z", "normal.zip"} {
		t.Run(name, func(t *testing.T) {
			root, addons := setupArchiveProcessingPaths(t)
			payload := archivePayload()
			var data []byte
			if name == "normal.zip" || strings.Contains(name, "-stored") {
				entries := []archiveEntry{
					{"root.vpk", payload}, {"nested/中文 地图.VPK", payload},
					{"nested/empty.vpk", nil}, {"nested/deeper/" + strings.Repeat("long", 55) + ".vpk", payload},
					{"nested/readme.txt", []byte("ignored")}, {"emptydir/", nil},
				}
				if name == "normal.zip" {
					data = testZIP(t, entries)
				} else if name == "rar4-stored.rar" {
					data = testRAR(4, entries)
				} else {
					data = testRAR(5, entries)
				}
			} else {
				data = archiveFixture(t, name)
			}
			path := writeTestArchive(t, root, name, data)
			got, err := ProcessFile(path)
			if err != nil {
				t.Fatalf("ProcessFile(%s): %v", name, err)
			}
			want := []string{"root.vpk", "中文_地图.vpk", "empty.vpk", strings.Repeat("long", 45) + ".vpk"}
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("extracted = %q, want %q", got, want)
			}
			for _, file := range want {
				content, err := os.ReadFile(filepath.Join(addons, file))
				if err != nil {
					t.Fatal(err)
				}
				if file == "empty.vpk" {
					if len(content) != 0 {
						t.Fatal("empty archive member gained content")
					}
					continue
				}
				if !bytes.Equal(content, payload) {
					t.Fatalf("extracted bytes differ for %s", file)
				}
				if _, err := vpk.ReadArchive(bytes.NewReader(content)); err != nil {
					t.Fatalf("extracted VPK is invalid: %v", err)
				}
			}
			mapList, err := os.ReadFile(consts.MapListFilePath)
			if err != nil {
				t.Fatal(err)
			}
			listed := strings.Fields(string(mapList))
			slices.Sort(listed)
			if !slices.Equal(listed, want) {
				t.Fatalf("maplist = %q, want %q", listed, want)
			}
			if _, err := os.Stat(filepath.Join(addons, "readme.txt")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("non-VPK was extracted: %v", err)
			}
			assertArchiveTempClean(t, addons)
			assertArchiveCanBeRenamed(t, path)
		})
	}
}

func TestProcessRarArchiveNamesStayInsideAddons(t *testing.T) {
	for _, format := range []int{4, 5} {
		t.Run(fmt.Sprintf("rar%d", format), func(t *testing.T) {
			root, addons := setupArchiveProcessingPaths(t)
			payload := archivePayload()
			outside := filepath.Join(root, "absolute.vpk")
			if err := os.WriteFile(outside, []byte("sentinel"), 0644); err != nil {
				t.Fatal(err)
			}
			entries := []archiveEntry{
				{"../traversal.vpk", payload}, {"..\\backslash.vpk", payload},
				{outside, payload}, {"nested/CON.vpk", payload},
			}
			got, err := ProcessFile(writeTestArchive(t, root, "names.rar", testRAR(format, entries)))
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"traversal.vpk", "backslash.vpk", "absolute.vpk", "CON_.vpk"}
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("sanitized names = %q, want %q", got, want)
			}
			for _, name := range want {
				content, err := os.ReadFile(filepath.Join(addons, name))
				if err != nil || !bytes.Equal(content, payload) {
					t.Fatalf("bad confined output %q: %v", name, err)
				}
			}
			content, err := os.ReadFile(outside)
			if err != nil || string(content) != "sentinel" {
				t.Fatalf("absolute path overwrote sentinel: %q, %v", content, err)
			}
			for _, name := range []string{"traversal.vpk", "backslash.vpk"} {
				if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("member escaped addons: %s, %v", name, err)
				}
			}
			assertArchiveTempClean(t, addons)
		})
	}
}

func TestProcessRarArchiveRejectsInvalidInput(t *testing.T) {
	for _, format := range []int{4, 5} {
		prefix := fmt.Sprintf("rar%d", format)
		payload := archivePayload()
		stored := testRAR(format, []archiveEntry{{"payload.vpk", payload}})
		dataOffset := bytes.Index(stored, payload)
		if dataOffset < 0 {
			t.Fatal("stored fixture does not contain the expected VPK bytes")
		}
		badCRC := bytes.Clone(stored)
		badCRC[dataOffset+len(payload)/2] ^= 1
		badHeader := bytes.Clone(stored)
		badHeader[7+format-4] ^= 1
		cases := []struct {
			name    string
			data    []byte
			fixture string
			cause   error
		}{
			{name: "empty"},
			{name: "not-rar", data: []byte("this is not a RAR archive"), cause: rardecode.ErrNoSig},
			{name: "truncated-header", data: stored[:10]},
			{name: "truncated-payload", data: stored[:dataOffset+len(payload)/2]},
			{name: "header-crc", data: badHeader},
			{name: "file-crc", data: badCRC, cause: rardecode.ErrBadFileChecksum},
			{name: "password", fixture: prefix + "-password.rar", cause: rardecode.ErrArchivedFileEncrypted},
			{name: "headers-encrypted", fixture: prefix + "-headers-encrypted.rar", cause: rardecode.ErrArchiveEncrypted},
			{name: "no-vpk", data: testRAR(format, []archiveEntry{{"readme.txt", []byte("No VPK in this archive.")}})},
			{name: "no-entries", data: testRAR(format, nil)},
		}
		for _, tc := range cases {
			t.Run(prefix+"/"+tc.name, func(t *testing.T) {
				root, addons := setupArchiveProcessingPaths(t)
				data := tc.data
				if tc.fixture != "" {
					data = archiveFixture(t, tc.fixture)
				}
				path := writeTestArchive(t, root, "invalid.rar", data)
				files, err := ProcessFile(path)
				if err == nil || len(files) != 0 {
					t.Fatalf("invalid archive succeeded: files=%q err=%v", files, err)
				}
				if tc.cause != nil && !errors.Is(err, tc.cause) {
					t.Fatalf("error lost cause %v: %v", tc.cause, err)
				}
				assertNoExtractedVPKs(t, addons)
				assertArchiveTempClean(t, addons)
				assertArchiveCanBeRenamed(t, path)
			})
		}
	}
}

func TestProcessRarArchiveRejectsMultiVolumeEvenWhenPartsExist(t *testing.T) {
	for _, format := range []int{4, 5} {
		for _, withParts := range []bool{false, true} {
			t.Run(fmt.Sprintf("rar%d/parts-present=%t", format, withParts), func(t *testing.T) {
				root, addons := setupArchiveProcessingPaths(t)
				prefix := fmt.Sprintf("rar%d-multivol.part", format)
				first := writeTestArchive(t, root, prefix+"1.rar", archiveFixture(t, prefix+"1.rar"))
				if withParts {
					for part := 2; part <= 5; part++ {
						name := fmt.Sprintf("%s%d.rar", prefix, part)
						writeTestArchive(t, root, name, archiveFixture(t, name))
					}
				}
				if _, err := ProcessFile(first); err == nil {
					t.Fatal("single-volume entry point unexpectedly accepted split archive")
				} else if !errors.Is(err, rardecode.ErrMultiVolume) {
					t.Fatalf("unexpected split-archive error: %v", err)
				}
				assertNoExtractedVPKs(t, addons)
				assertArchiveTempClean(t, addons)
				assertArchiveCanBeRenamed(t, first)
			})
		}
	}
}

func TestProcessRarArchiveDoesNotOverwriteDuplicateNames(t *testing.T) {
	for _, format := range []int{4, 5} {
		t.Run(fmt.Sprint(format), func(t *testing.T) {
			root, addons := setupArchiveProcessingPaths(t)
			payload := archivePayload()
			data := testRAR(format, []archiveEntry{{"first/map.vpk", payload}, {"second/map.vpk", []byte("replacement")}})
			if _, err := ProcessFile(writeTestArchive(t, root, "duplicate.rar", data)); err == nil || !strings.Contains(err.Error(), "重名") {
				t.Fatalf("duplicate should fail: %v", err)
			}
			assertNoExtractedVPKs(t, addons)
			mapList, err := os.ReadFile(consts.MapListFilePath)
			if err != nil || len(mapList) != 0 {
				t.Fatalf("duplicate archive changed maplist: %q, %v", mapList, err)
			}
			assertArchiveTempClean(t, addons)
		})
	}
}

func TestProcessRarArchivePropagatesOutputErrors(t *testing.T) {
	for _, format := range []int{4, 5} {
		t.Run(fmt.Sprint(format), func(t *testing.T) {
			root, addons := setupArchiveProcessingPaths(t)
			consts.MapListFilePath = filepath.Join(root, "maplist-is-a-directory")
			if err := os.MkdirAll(consts.MapListFilePath, 0755); err != nil {
				t.Fatal(err)
			}
			name := fmt.Sprintf("rar%d-stored.rar", format)
			path := writeTestArchive(t, root, name, testRAR(format, []archiveEntry{{"payload.vpk", archivePayload()}}))
			if _, err := ProcessFile(path); err == nil || !strings.Contains(err.Error(), "地图记录") {
				t.Fatalf("output failure was not propagated: %v", err)
			}
			assertNoExtractedVPKs(t, addons)
			assertArchiveTempClean(t, addons)
			assertArchiveCanBeRenamed(t, path)
		})
	}
	t.Run("missing-input", func(t *testing.T) {
		root, _ := setupArchiveProcessingPaths(t)
		if _, err := ProcessFile(filepath.Join(root, "missing.rar")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing input cause not preserved: %v", err)
		}
	})
}

func TestRarReaderEnforcesSmallDictionaryLimit(t *testing.T) {
	// Exercise the v2 limit with a 1 MiB dictionary and 256 KiB limit, never a huge allocation.
	for _, format := range []int{4, 5} {
		t.Run(fmt.Sprint(format), func(t *testing.T) {
			data := archiveFixture(t, fmt.Sprintf("rar%d-dictionary.rar", format))
			defaultReader, err := rardecode.NewReader(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := defaultReader.Next(); err != nil {
				t.Fatal(err)
			}
			size, err := io.Copy(io.Discard, defaultReader)
			if err != nil || size <= 256<<10 || size >= 1<<20 {
				t.Fatalf("bounded dictionary fixture is invalid: size=%d err=%v", size, err)
			}
			r, err := rardecode.NewReader(bytes.NewReader(data), rardecode.MaxDictionarySize(256<<10))
			if err != nil {
				t.Fatal(err)
			}
			for {
				_, err = r.Next()
				if err != nil {
					break
				}
				if _, err = io.Copy(io.Discard, r); err != nil {
					break
				}
			}
			if !errors.Is(err, rardecode.ErrDictionaryTooLarge) {
				t.Fatalf("small dictionary limit not enforced: %v", err)
			}
		})
	}
}

func TestProcessRarArchiveClosesInputAfterRepeatedErrors(t *testing.T) {
	root, addons := setupArchiveProcessingPaths(t)
	path := writeTestArchive(t, root, "broken.rar", []byte("invalid archive"))
	before, _ := os.ReadDir("/proc/self/fd")
	for i := 0; i < 64; i++ {
		if _, err := ProcessFile(path); err == nil {
			t.Fatal("invalid archive succeeded")
		}
		assertArchiveCanBeRenamed(t, path)
	}
	if runtime.GOOS == "linux" && len(before) != 0 {
		after, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Fatal(err)
		}
		if len(after) > len(before)+2 {
			t.Fatalf("input file descriptors leaked: before=%d after=%d", len(before), len(after))
		}
	}
	assertArchiveTempClean(t, addons)
}

func TestProcessZipAnd7zRejectCorruptInput(t *testing.T) {
	for _, name := range []string{"broken.zip", "broken.7z"} {
		t.Run(name, func(t *testing.T) {
			root, addons := setupArchiveProcessingPaths(t)
			if _, err := ProcessFile(writeTestArchive(t, root, name, []byte("not an archive"))); err == nil {
				t.Fatal("invalid archive succeeded")
			}
			assertNoExtractedVPKs(t, addons)
			assertArchiveTempClean(t, addons)
		})
	}
	t.Run("truncated-7z", func(t *testing.T) {
		root, addons := setupArchiveProcessingPaths(t)
		data := archiveFixture(t, "normal.7z")
		if _, err := ProcessFile(writeTestArchive(t, root, "truncated.7z", data[:len(data)/2])); err == nil {
			t.Fatal("truncated 7z succeeded")
		}
		assertNoExtractedVPKs(t, addons)
		assertArchiveTempClean(t, addons)
	})
	t.Run("corrupt-7z-payload", func(t *testing.T) {
		root, addons := setupArchiveProcessingPaths(t)
		data := bytes.Clone(archiveFixture(t, "normal.7z"))
		data[32] ^= 0x40
		if _, err := ProcessFile(writeTestArchive(t, root, "corrupt.7z", data)); err == nil {
			t.Fatal("corrupt 7z payload succeeded")
		}
		assertNoExtractedVPKs(t, addons)
		assertArchiveTempClean(t, addons)
	})
	t.Run("zip-crc", func(t *testing.T) {
		root, addons := setupArchiveProcessingPaths(t)
		payload := archivePayload()
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		f, err := w.CreateHeader(&zip.FileHeader{Name: "map.vpk", Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		data := bytes.Clone(buf.Bytes())
		offset := bytes.Index(data, payload)
		if offset < 0 {
			t.Fatal("ZIP payload not found")
		}
		data[offset+len(payload)/2] ^= 1
		if _, err := ProcessFile(writeTestArchive(t, root, "corrupt.zip", data)); err == nil || !strings.Contains(err.Error(), "checksum") {
			t.Fatalf("ZIP checksum error not propagated: %v", err)
		}
		assertNoExtractedVPKs(t, addons)
		assertArchiveTempClean(t, addons)
	})
}

func TestProcessRarArchiveRejectsWholeArchiveAfterLaterError(t *testing.T) {
	// Preserve the batch import behavior from the current main branch.
	for _, format := range []int{4, 5} {
		t.Run(fmt.Sprint(format), func(t *testing.T) {
			root, addons := setupArchiveProcessingPaths(t)
			payload := archivePayload()
			data := testRAR(format, []archiveEntry{{"first.vpk", payload}, {"broken.vpk", payload}})
			offset := bytes.LastIndex(data, payload)
			if offset < 0 {
				t.Fatal("second RAR payload not found")
			}
			data[offset+len(payload)/2] ^= 1
			path := writeTestArchive(t, root, "partial.rar", data)
			if _, err := ProcessFile(path); !errors.Is(err, rardecode.ErrBadFileChecksum) {
				t.Fatalf("later member error not propagated: %v", err)
			}
			assertNoExtractedVPKs(t, addons)
			mapList, err := os.ReadFile(consts.MapListFilePath)
			if err != nil || len(mapList) != 0 {
				t.Fatalf("failed archive changed maplist: %q, %v", mapList, err)
			}
			assertArchiveTempClean(t, addons)
			assertArchiveCanBeRenamed(t, path)
		})
	}
}

type archiveEntry struct {
	name string
	data []byte
}

func setupArchiveProcessingPaths(t *testing.T) (string, string) {
	t.Helper()
	root, _ := setupMapImportTest(t)
	addons := filepath.Join(root, "controlled", "addons")
	if err := os.MkdirAll(addons, 0755); err != nil {
		t.Fatal(err)
	}
	consts.AddonsBasePath = addons
	consts.MapListFilePath = filepath.Join(addons, "maplist.txt")
	if err := os.WriteFile(consts.MapListFilePath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	return root, addons
}

func archiveFixture(t *testing.T, name string) []byte {
	t.Helper()
	// Optional compressed/encrypted corpus is intentionally gitignored. Core RAR/ZIP
	// cases generate their input in memory and still run in a fresh checkout.
	path := os.Getenv("L4D2_ARCHIVE_FIXTURE_ZIP")
	if path == "" {
		path = filepath.Join("testdata", "archive-fixtures.zip")
	}
	archive, err := zip.OpenReader(path)
	if errors.Is(err, os.ErrNotExist) && os.Getenv("L4D2_ARCHIVE_FIXTURE_ZIP") == "" {
		t.Skip("optional archive corpus absent; set L4D2_ARCHIVE_FIXTURE_ZIP to run this case")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	f, err := archive.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func archivePayload() []byte {
	content := bytes.Repeat([]byte("// archive regression fixture\n"), 128)
	tree := []byte("txt\x00 \x00addoninfo\x00")
	tree = binary.LittleEndian.AppendUint32(tree, crc32.ChecksumIEEE(content))
	tree = binary.LittleEndian.AppendUint16(tree, 0)
	tree = binary.LittleEndian.AppendUint16(tree, 0x7fff)
	tree = binary.LittleEndian.AppendUint32(tree, 0)
	tree = binary.LittleEndian.AppendUint32(tree, uint32(len(content)))
	tree = binary.LittleEndian.AppendUint16(tree, 0xffff)
	tree = append(tree, 0, 0, 0)
	out := binary.LittleEndian.AppendUint32(nil, 0x55aa1234)
	out = binary.LittleEndian.AppendUint32(out, 1)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(tree)))
	out = append(out, tree...)
	return append(out, content...)
}

func writeTestArchive(t *testing.T, root, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertArchiveTempClean(t *testing.T, addons string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(addons, logic.VPKTrimTempDirName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("archive temporary files leaked: %v", entries)
	}
}

func assertNoExtractedVPKs(t *testing.T, addons string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(addons, "*.vpk"))
	if err != nil || len(files) != 0 {
		t.Fatalf("unexpected extracted files: %q, %v", files, err)
	}
}

func assertArchiveCanBeRenamed(t *testing.T, path string) {
	t.Helper()
	if err := os.Rename(path, path+".closed"); err != nil {
		t.Fatalf("archive input remains locked: %v", err)
	}
	if err := os.Rename(path+".closed", path); err != nil {
		t.Fatal(err)
	}
}

func testZIP(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		f, err := w.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Minimal stored archives let path and malformed-header tests use platform-native
// absolute paths without ever targeting a location outside t.TempDir(). Compressed,
// Unicode, encrypted, solid, and split streams use independently generated fixtures.
func testRAR(format int, entries []archiveEntry) []byte {
	if format == 4 {
		out := []byte("Rar!\x1a\x07\x00")
		block := func(kind byte, flags uint16, body []byte) {
			header := []byte{0, 0, kind}
			header = binary.LittleEndian.AppendUint16(header, flags)
			header = binary.LittleEndian.AppendUint16(header, uint16(7+len(body)))
			header = append(header, body...)
			binary.LittleEndian.PutUint16(header, uint16(crc32.ChecksumIEEE(header[2:])))
			out = append(out, header...)
		}
		block(0x73, 0, make([]byte, 6))
		for _, e := range entries {
			body := binary.LittleEndian.AppendUint32(nil, uint32(len(e.data)))
			body = binary.LittleEndian.AppendUint32(body, uint32(len(e.data)))
			body = append(body, 2)
			body = binary.LittleEndian.AppendUint32(body, crc32.ChecksumIEEE(e.data))
			body = binary.LittleEndian.AppendUint32(body, 0)
			body = append(body, 20, 0x30)
			body = binary.LittleEndian.AppendUint16(body, uint16(len(e.name)))
			body = binary.LittleEndian.AppendUint32(body, 0x20)
			block(0x74, 0x8000, append(body, e.name...))
			out = append(out, e.data...)
		}
		block(0x7b, 0, nil)
		return out
	}
	out := []byte("Rar!\x1a\x07\x01\x00")
	block := func(body []byte) {
		header := binary.AppendUvarint(nil, uint64(len(body)))
		header = append(header, body...)
		out = binary.LittleEndian.AppendUint32(out, crc32.ChecksumIEEE(header))
		out = append(out, header...)
	}
	block([]byte{1, 0, 0})
	for _, e := range entries {
		body := []byte{2, 2}
		body = binary.AppendUvarint(body, uint64(len(e.data)))
		body = append(body, 4)
		body = binary.AppendUvarint(body, uint64(len(e.data)))
		body = append(body, 0x20)
		body = binary.LittleEndian.AppendUint32(body, crc32.ChecksumIEEE(e.data))
		body = append(body, 0, 0)
		body = binary.AppendUvarint(body, uint64(len(e.name)))
		block(append(body, e.name...))
		out = append(out, e.data...)
	}
	block([]byte{5, 0, 0})
	return out
}
