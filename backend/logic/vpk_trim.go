package logic

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"l4d2-manager-next/consts"

	"github.com/google/uuid"
)

const (
	VPKTrimTempDirName = ".vpk_trim_temp"

	rawVPKMagic       = 0x55aa1234
	rawVPKVersion1    = 1
	rawVPKSelfArchive = 0x7fff
	rawVPKTerminator  = 0xffff
)

var errRawVPKTrimUnsupported = errors.New("unsupported vpk layout")

func IsVPKTrimUnsupported(err error) bool {
	return errors.Is(err, errRawVPKTrimUnsupported)
}

func CleanVPKTrimTemp() {
	os.RemoveAll(filepath.Join(consts.AddonsBasePath, VPKTrimTempDirName))
}

func TrimVPKForServer(sourcePath string) (string, func(), error) {
	tempRoot := filepath.Join(consts.AddonsBasePath, VPKTrimTempDirName)
	if err := os.MkdirAll(tempRoot, 0755); err != nil {
		return "", nil, fmt.Errorf("创建VPK精简临时目录失败: %w", err)
	}

	tempDir := filepath.Join(tempRoot, uuid.NewString())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", nil, fmt.Errorf("创建VPK精简任务目录失败: %w", err)
	}
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	trimmedPath := filepath.Join(tempDir, "trimmed.vpk")
	if _, _, err := trimRawSingleFileVPK(sourcePath, trimmedPath); err != nil {
		cleanup()
		if errors.Is(err, errRawVPKTrimUnsupported) {
			return "", nil, fmt.Errorf("当前仅支持VPK v1单文件地图精简: %w", err)
		}
		return "", nil, fmt.Errorf("VPK内部精简失败: %w", err)
	}

	if err := validateTrimmedVPK(trimmedPath); err != nil {
		cleanup()
		return "", nil, err
	}

	return trimmedPath, cleanup, nil
}

func validateTrimmedVPK(trimmedPath string) error {
	if info, err := os.Stat(trimmedPath); err != nil {
		return fmt.Errorf("读取精简VPK失败: %w", err)
	} else if info.Size() == 0 {
		return fmt.Errorf("精简VPK为空")
	}
	return nil
}

func shouldRemoveVPKTrimFile(relPath string) bool {
	rel := strings.ToLower(filepath.ToSlash(relPath))
	ext := strings.ToLower(path.Ext(rel))

	switch ext {
	case ".vmf", ".vmx":
		return true
	}

	if strings.HasPrefix(rel, "materials/") {
		return ext == ".vtf"
	}
	if strings.HasPrefix(rel, "sound/") || strings.HasPrefix(rel, "sounds/") {
		return ext == ".mp3" || ext == ".wav"
	}
	if strings.HasPrefix(rel, "models/") {
		return ext == ".vvd" || ext == ".vtx"
	}

	return false
}

type rawVPKHeader struct {
	Magic    uint32
	Version  uint32
	TreeSize uint32
}

type rawVPKChunk struct {
	ArchiveIndex uint16
	EntryOffset  uint32
	EntryLength  uint32
}

type rawVPKEntry struct {
	Ext      []byte
	Dir      []byte
	Base     []byte
	CRC      uint32
	Metadata []byte
	Chunks   []rawVPKChunk
}

func trimRawSingleFileVPK(sourcePath, trimmedPath string) (int, int64, error) {
	in, err := os.Open(sourcePath)
	if err != nil {
		return 0, 0, err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return 0, 0, err
	}

	var header rawVPKHeader
	if err := binary.Read(in, binary.LittleEndian, &header); err != nil {
		return 0, 0, fmt.Errorf("读取VPK头失败: %w", err)
	}
	if header.Magic != rawVPKMagic {
		return 0, 0, fmt.Errorf("%w: invalid magic %08x", errRawVPKTrimUnsupported, header.Magic)
	}
	if header.Version != rawVPKVersion1 {
		return 0, 0, fmt.Errorf("%w: version %d", errRawVPKTrimUnsupported, header.Version)
	}

	tree := make([]byte, header.TreeSize)
	if _, err := io.ReadFull(in, tree); err != nil {
		return 0, 0, fmt.Errorf("读取VPK目录树失败: %w", err)
	}

	entries, err := parseRawVPKTree(tree)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %v", errRawVPKTrimUnsupported, err)
	}

	dataTempPath := trimmedPath + ".data"
	_ = os.Remove(dataTempPath)
	defer os.Remove(dataTempPath)

	dataFile, err := os.OpenFile(dataTempPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return 0, 0, fmt.Errorf("创建VPK数据临时文件失败: %w", err)
	}
	defer dataFile.Close()

	headerSize := int64(binary.Size(header))
	oldDataBase := headerSize + int64(header.TreeSize)
	var dataOffset int64
	kept := make([]rawVPKEntry, 0, len(entries))
	var removedCount int
	var removedBytes int64

	for _, entry := range entries {
		if shouldRemoveRawVPKEntry(entry) {
			removedCount++
			removedBytes += rawVPKEntrySize(entry)
			continue
		}

		newEntry := rawVPKEntry{
			Ext:      entry.Ext,
			Dir:      entry.Dir,
			Base:     entry.Base,
			CRC:      entry.CRC,
			Metadata: entry.Metadata,
			Chunks:   make([]rawVPKChunk, 0, len(entry.Chunks)),
		}

		for _, chunk := range entry.Chunks {
			if chunk.ArchiveIndex != rawVPKSelfArchive {
				return 0, 0, fmt.Errorf("%w: external archive %d", errRawVPKTrimUnsupported, chunk.ArchiveIndex)
			}
			if dataOffset > int64(^uint32(0)) {
				return 0, 0, fmt.Errorf("VPK数据超过单文件偏移上限")
			}
			if oldDataBase+int64(chunk.EntryOffset)+int64(chunk.EntryLength) > info.Size() {
				return 0, 0, fmt.Errorf("%w: chunk exceeds file size", errRawVPKTrimUnsupported)
			}

			newChunk := rawVPKChunk{
				ArchiveIndex: rawVPKSelfArchive,
				EntryOffset:  uint32(dataOffset),
				EntryLength:  chunk.EntryLength,
			}
			if chunk.EntryLength > 0 {
				reader := io.NewSectionReader(in, oldDataBase+int64(chunk.EntryOffset), int64(chunk.EntryLength))
				if _, err := io.CopyN(dataFile, reader, int64(chunk.EntryLength)); err != nil {
					return 0, 0, fmt.Errorf("复制VPK条目数据失败: %w", err)
				}
				dataOffset += int64(chunk.EntryLength)
			}
			newEntry.Chunks = append(newEntry.Chunks, newChunk)
		}
		kept = append(kept, newEntry)
	}

	newTree, err := buildRawVPKTree(kept)
	if err != nil {
		return 0, 0, err
	}
	header.TreeSize = uint32(len(newTree))

	out, err := os.OpenFile(trimmedPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return 0, 0, fmt.Errorf("创建精简VPK失败: %w", err)
	}
	defer out.Close()

	if err := binary.Write(out, binary.LittleEndian, &header); err != nil {
		return 0, 0, fmt.Errorf("写入VPK头失败: %w", err)
	}
	if _, err := out.Write(newTree); err != nil {
		return 0, 0, fmt.Errorf("写入VPK目录树失败: %w", err)
	}
	if _, err := dataFile.Seek(0, io.SeekStart); err != nil {
		return 0, 0, fmt.Errorf("读取VPK数据临时文件失败: %w", err)
	}
	if _, err := io.Copy(out, dataFile); err != nil {
		return 0, 0, fmt.Errorf("写入VPK数据失败: %w", err)
	}
	if err := out.Sync(); err != nil {
		return 0, 0, fmt.Errorf("同步精简VPK失败: %w", err)
	}

	return removedCount, removedBytes, nil
}

func parseRawVPKTree(tree []byte) ([]rawVPKEntry, error) {
	var entries []rawVPKEntry
	pos := 0
	for {
		ext, next, err := readRawVPKString(tree, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		if len(ext) == 0 {
			break
		}

		for {
			dir, next, err := readRawVPKString(tree, pos)
			if err != nil {
				return nil, err
			}
			pos = next
			if len(dir) == 0 {
				break
			}

			for {
				base, next, err := readRawVPKString(tree, pos)
				if err != nil {
					return nil, err
				}
				pos = next
				if len(base) == 0 {
					break
				}
				if pos+6 > len(tree) {
					return nil, io.ErrUnexpectedEOF
				}

				entry := rawVPKEntry{
					Ext:  append([]byte(nil), ext...),
					Dir:  append([]byte(nil), dir...),
					Base: append([]byte(nil), base...),
					CRC:  binary.LittleEndian.Uint32(tree[pos:]),
				}
				pos += 4
				metadataBytes := int(binary.LittleEndian.Uint16(tree[pos:]))
				pos += 2

				for {
					if pos+2 > len(tree) {
						return nil, io.ErrUnexpectedEOF
					}
					archiveIndex := binary.LittleEndian.Uint16(tree[pos:])
					pos += 2
					if archiveIndex == rawVPKTerminator {
						break
					}
					if pos+8 > len(tree) {
						return nil, io.ErrUnexpectedEOF
					}
					entry.Chunks = append(entry.Chunks, rawVPKChunk{
						ArchiveIndex: archiveIndex,
						EntryOffset:  binary.LittleEndian.Uint32(tree[pos:]),
						EntryLength:  binary.LittleEndian.Uint32(tree[pos+4:]),
					})
					pos += 8
				}
				if len(entry.Chunks) == 0 {
					return nil, fmt.Errorf("entry has no data chunks")
				}
				if pos+metadataBytes > len(tree) {
					return nil, io.ErrUnexpectedEOF
				}
				entry.Metadata = append([]byte(nil), tree[pos:pos+metadataBytes]...)
				pos += metadataBytes
				entries = append(entries, entry)
			}
		}
	}
	if pos != len(tree) {
		return nil, fmt.Errorf("tree has %d trailing bytes", len(tree)-pos)
	}
	return entries, nil
}

func readRawVPKString(data []byte, pos int) ([]byte, int, error) {
	start := pos
	for pos < len(data) && data[pos] != 0 {
		pos++
	}
	if pos >= len(data) {
		return nil, pos, io.ErrUnexpectedEOF
	}
	return data[start:pos], pos + 1, nil
}

func buildRawVPKTree(entries []rawVPKEntry) ([]byte, error) {
	var tree bytes.Buffer
	var lastExt []byte
	var lastDir []byte

	for _, entry := range entries {
		if lastExt == nil || !bytes.Equal(entry.Ext, lastExt) {
			if lastExt != nil {
				tree.WriteByte(0)
				tree.WriteByte(0)
			}
			tree.Write(entry.Ext)
			tree.WriteByte(0)
			tree.Write(entry.Dir)
			tree.WriteByte(0)
			lastExt = append(lastExt[:0], entry.Ext...)
			lastDir = append(lastDir[:0], entry.Dir...)
		} else if !bytes.Equal(entry.Dir, lastDir) {
			tree.WriteByte(0)
			tree.Write(entry.Dir)
			tree.WriteByte(0)
			lastDir = append(lastDir[:0], entry.Dir...)
		}

		tree.Write(entry.Base)
		tree.WriteByte(0)
		if err := binary.Write(&tree, binary.LittleEndian, entry.CRC); err != nil {
			return nil, err
		}
		if len(entry.Metadata) > int(^uint16(0)) {
			return nil, fmt.Errorf("VPK预载数据过大")
		}
		if err := binary.Write(&tree, binary.LittleEndian, uint16(len(entry.Metadata))); err != nil {
			return nil, err
		}
		for _, chunk := range entry.Chunks {
			if err := binary.Write(&tree, binary.LittleEndian, chunk); err != nil {
				return nil, err
			}
		}
		if err := binary.Write(&tree, binary.LittleEndian, uint16(rawVPKTerminator)); err != nil {
			return nil, err
		}
		tree.Write(entry.Metadata)
	}

	if lastExt != nil {
		tree.WriteByte(0)
		tree.WriteByte(0)
		tree.WriteByte(0)
	}
	return tree.Bytes(), nil
}

func shouldRemoveRawVPKEntry(entry rawVPKEntry) bool {
	name := asciiLowerRawVPKName(entry)
	ext := rawPathExt(name)

	switch string(ext) {
	case ".vmf", ".vmx":
		return true
	}

	if bytes.HasPrefix(name, []byte("materials/")) {
		return bytes.Equal(ext, []byte(".vtf"))
	}
	if bytes.HasPrefix(name, []byte("sound/")) || bytes.HasPrefix(name, []byte("sounds/")) {
		return bytes.Equal(ext, []byte(".mp3")) || bytes.Equal(ext, []byte(".wav"))
	}
	if bytes.HasPrefix(name, []byte("models/")) {
		return bytes.Equal(ext, []byte(".vvd")) || bytes.Equal(ext, []byte(".vtx"))
	}
	return false
}

func asciiLowerRawVPKName(entry rawVPKEntry) []byte {
	name := make([]byte, 0, len(entry.Dir)+len(entry.Base)+len(entry.Ext)+2)
	if !bytes.Equal(entry.Dir, []byte(" ")) {
		name = append(name, entry.Dir...)
		name = append(name, '/')
	}
	if !bytes.Equal(entry.Base, []byte(" ")) {
		name = append(name, entry.Base...)
	}
	if !bytes.Equal(entry.Ext, []byte(" ")) {
		name = append(name, '.')
		name = append(name, entry.Ext...)
	}
	for i, b := range name {
		if b == '\\' {
			name[i] = '/'
			continue
		}
		if b >= 'A' && b <= 'Z' {
			name[i] = b + ('a' - 'A')
		}
	}
	return name
}

func rawPathExt(name []byte) []byte {
	slash := bytes.LastIndexByte(name, '/')
	dot := bytes.LastIndexByte(name, '.')
	if dot <= slash {
		return nil
	}
	return name[dot:]
}

func rawVPKEntrySize(entry rawVPKEntry) int64 {
	size := int64(len(entry.Metadata))
	for _, chunk := range entry.Chunks {
		size += int64(chunk.EntryLength)
	}
	return size
}
