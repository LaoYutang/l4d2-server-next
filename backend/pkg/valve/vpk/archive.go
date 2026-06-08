// Package vpk provides a reader for the Valve PacK archive format.
package vpk

import (
	"bufio"
	"bytes"
	"crypto"
	"crypto/md5"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
)

// Archive is the directory data read from a VPK.
type Archive struct {
	Header
	Header2
	Files  []File
	Hashes []ChunkHash
	PubKey *rsa.PublicKey
}

// ReadArchive reads and validates a VPK file.
//
// Individual file and chunk hashes are not validated in this pass.
//
// If a signed VPK is expected, check to make sure the PubKey field contains
// a trusted public key.
func ReadArchive(r io.Reader) (*Archive, error) {
	var a Archive

	sigCheck, fileCheck := sha256.New(), md5.New()
	r = io.TeeReader(io.TeeReader(r, sigCheck), fileCheck)

	err := a.readHeader(r)
	if err != nil {
		return nil, err
	}

	treeHash, err := a.readTree(r)
	if err != nil {
		return nil, err
	}

	if a.Version >= version2 {
		err = a.readHashes(r, treeHash, sigCheck, fileCheck)
		if err != nil {
			return nil, err
		}
	}

	return &a, nil
}

func (a *Archive) readHeader(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &a.Header)
	if err != nil {
		return fmt.Errorf("vpk: reading header: %w", err)
	}

	if a.Magic != Magic {
		return fmt.Errorf("vpk: invalid magic: %08x", a.Magic)
	}

	if a.Version < version1 {
		return fmt.Errorf("vpk: invalid version: %d", a.Version)
	}

	if a.Version > version2 {
		return fmt.Errorf("vpk: unsupported version: %d", a.Version)
	}

	if a.Version >= version2 {
		err = binary.Read(r, binary.LittleEndian, &a.Header2)
		if err != nil {
			return fmt.Errorf("vpk: reading header: %w", err)
		}

		if int(a.ChunkHashesSize)%binary.Size(ChunkHash{}) != 0 {
			return fmt.Errorf("vpk: chunk hashes section has invalid size %d", a.ChunkHashesSize)
		}

		if int(a.SelfHashesSize) != binary.Size(selfHashes{}) {
			return fmt.Errorf("vpk: self hashes section has invalid size %d", a.SelfHashesSize)
		}
	}

	return nil
}

func (a *Archive) readTree(r io.Reader) ([]byte, error) {
	var err error

	check := md5.New()
	tr := bufio.NewReader(io.TeeReader(io.LimitReader(r, int64(a.TreeSize)), check))

	var f File

	for {
		f.Ext, err = tr.ReadString(0)
		if err != nil {
			return nil, fmt.Errorf("vpk: reading tree: %w", err)
		}

		f.Ext = f.Ext[:len(f.Ext)-1]

		if f.Ext == "" {
			break
		}

		for {
			f.Dir, err = tr.ReadString(0)
			if err != nil {
				return nil, fmt.Errorf("vpk: reading tree: %w", err)
			}

			f.Dir = f.Dir[:len(f.Dir)-1]

			if f.Dir == "" {
				break
			}

			for {
				f.Base, err = tr.ReadString(0)
				if err != nil {
					return nil, fmt.Errorf("vpk: reading tree: %w", err)
				}

				f.Base = f.Base[:len(f.Base)-1]

				if f.Base == "" {
					break
				}

				err = binary.Read(tr, binary.LittleEndian, &f.CRC)
				if err != nil {
					return nil, fmt.Errorf("vpk: reading tree: %w", err)
				}

				err = binary.Read(tr, binary.LittleEndian, &f.MetadataBytes)
				if err != nil {
					return nil, fmt.Errorf("vpk: reading tree: %w", err)
				}

				f.DataLocation = nil

				for {
					var chunk DataChunk

					err = binary.Read(tr, binary.LittleEndian, &chunk.ArchiveIndex)
					if err != nil {
						return nil, fmt.Errorf("vpk: reading tree: %w", err)
					}

					if chunk.ArchiveIndex == terminator {
						if len(f.DataLocation) == 0 {
							return nil, fmt.Errorf("vpk: unexpected directory entry terminator")
						}

						break
					}

					err = binary.Read(tr, binary.LittleEndian, &chunk.EntryOffset)
					if err != nil {
						return nil, fmt.Errorf("vpk: reading tree: %w", err)
					}

					err = binary.Read(tr, binary.LittleEndian, &chunk.EntryLength)
					if err != nil {
						return nil, fmt.Errorf("vpk: reading tree: %w", err)
					}

					f.DataLocation = append(f.DataLocation, chunk)
				}

				if f.MetadataBytes == 0 {
					f.Metadata = nil
				} else {
					f.Metadata = make([]byte, f.MetadataBytes)
				}

				_, err = io.ReadFull(tr, f.Metadata)
				if err != nil {
					return nil, fmt.Errorf("vpk: reading tree: %w", err)
				}

				f.fileOffset = a.TreeSize + uint32(binary.Size(a.Header))

				a.Files = append(a.Files, f)
			}
		}
	}

	_, err = tr.ReadByte()
	if err == nil {
		return nil, fmt.Errorf("vpk: tree ended early")
	}

	if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("vpk: reading tree: %w", err)
	}

	return check.Sum(nil), nil
}

func (a *Archive) readHashes(r io.Reader, treeHash []byte, sigCheck, fileCheck hash.Hash) error {
	_, err := io.CopyN(io.Discard, r, int64(a.EmbeddedChunkSize))
	if err != nil {
		return fmt.Errorf("vpk: reading embedded data: %w", err)
	}

	hashCheck := md5.New()
	a.Hashes = make([]ChunkHash, int(a.ChunkHashesSize)/binary.Size(ChunkHash{}))

	err = binary.Read(io.TeeReader(r, hashCheck), binary.LittleEndian, a.Hashes)
	if err != nil {
		return fmt.Errorf("vpk: reading chunk hashes: %w", err)
	}

	var hashes selfHashes

	err = binary.Read(r, binary.LittleEndian, &hashes.TreeChecksum)
	if err != nil {
		return fmt.Errorf("vpk: reading self hashes: %w", err)
	}

	if !bytes.Equal(treeHash, hashes.TreeChecksum[:]) {
		return fmt.Errorf("vpk: checksum mismatch")
	}

	chunkHashesHash := hashCheck.Sum(nil)

	err = binary.Read(r, binary.LittleEndian, &hashes.ChunkHashesChecksum)
	if err != nil {
		return fmt.Errorf("vpk: reading self hashes: %w", err)
	}

	if !bytes.Equal(chunkHashesHash, hashes.ChunkHashesChecksum[:]) {
		return fmt.Errorf("vpk: checksum mismatch")
	}

	fileHash := fileCheck.Sum(nil)

	err = binary.Read(r, binary.LittleEndian, &hashes.FileChecksum)
	if err != nil {
		return fmt.Errorf("vpk: reading self hashes: %w", err)
	}

	if !bytes.Equal(fileHash, hashes.FileChecksum[:]) {
		return fmt.Errorf("vpk: checksum mismatch")
	}

	if a.SignatureSize != 0 {
		signatureHash := sigCheck.Sum(nil)

		var (
			signature fileSignature
			length    uint32
		)

		err = binary.Read(r, binary.LittleEndian, &length)
		if err != nil {
			return fmt.Errorf("vpk: reading signature: %w", err)
		}

		signature.PublicKey = make([]byte, length)

		_, err = io.ReadFull(r, signature.PublicKey)
		if err != nil {
			return fmt.Errorf("vpk: reading signature: %w", err)
		}

		err = binary.Read(r, binary.LittleEndian, &length)
		if err != nil {
			return fmt.Errorf("vpk: reading signature: %w", err)
		}

		signature.Signature = make([]byte, length)

		_, err = io.ReadFull(r, signature.Signature)
		if err != nil {
			return fmt.Errorf("vpk: reading signature: %w", err)
		}

		pubKey, err := x509.ParsePKIXPublicKey(signature.PublicKey)
		if err != nil {
			return fmt.Errorf("vpk: verifying signature: %w", err)
		}

		var ok bool

		a.PubKey, ok = pubKey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("vpk: not an RSA key: %T", pubKey)
		}

		err = rsa.VerifyPKCS1v15(a.PubKey, crypto.SHA256, signatureHash, signature.Signature)
		if err != nil {
			return fmt.Errorf("vpk: verifying signature: %w", err)
		}
	}

	return nil
}
