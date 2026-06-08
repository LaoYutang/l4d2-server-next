package vpk

const (
	// Magic is the "magic number" that every VPK directory must start with.
	Magic = 0x55aa1234

	version1    = 1
	version2    = 2
	selfArchive = 0x7fff
	terminator  = 0xffff
)

// Header is the VPK version 1 file format header.
type Header struct {
	// Must be equal to the Magic constant in this package.
	Magic uint32

	// VPK file format version.
	Version uint32

	// The size, in bytes, of the directory tree.
	TreeSize uint32
}

// Header2 is the additional data that follows Header in VPK version 2.
type Header2 struct {
	// How many bytes of file content are stored in this VPK file.
	EmbeddedChunkSize uint32

	// The size, in bytes, of the section containing MD5 checksums
	// for external archive content.
	ChunkHashesSize uint32

	// The size, in bytes, of the section containing MD5 checksums
	// for content in this file (should always be 48).
	SelfHashesSize uint32

	// The size, in bytes, of the section containing the public key
	// and signature.
	SignatureSize uint32
}

// File is metadata about a single file entry in a VPK.
type File struct {
	Dir  string
	Base string
	Ext  string

	DirEntry
	fileOffset uint32

	Metadata []byte
}

// DirEntry is the raw VPK directory data.
type DirEntry struct {
	// A 32bit CRC of the file's data.
	CRC uint32

	// The number of bytes contained in the index file.
	MetadataBytes uint16

	// The location of the contents of this file (minimum of 1 element).
	DataLocation []DataChunk
}

// DataChunk is the location of a file's data.
type DataChunk struct {
	// A zero based index of the archive this file's data is contained in.
	// If 0x7fff, the data follows the directory.
	ArchiveIndex uint16

	// If ArchiveIndex is 0x7fff, the offset of the file data relative to
	// the end of the directory (see the header for more details).
	//
	// Otherwise, the offset of the data from the start of the specified
	// archive.
	EntryOffset uint32

	// If zero, the entire file is stored in the preload data.
	// Otherwise, the number of bytes stored starting at EntryOffset.
	EntryLength uint32
}

// ChunkHash is a hash of an arbitrary section of a VPK archive set.
type ChunkHash struct {
	ArchiveIndex   uint32
	StartingOffset uint32
	Count          uint32
	MD5Checksum    [16]byte
}

type selfHashes struct {
	TreeChecksum        [16]byte
	ChunkHashesChecksum [16]byte
	FileChecksum        [16]byte
}

type fileSignature struct {
	PublicKey []byte
	Signature []byte
}
