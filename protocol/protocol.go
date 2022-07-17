// SPDX-License-Identifier: MIT
// Copyright (c) 2021 Brian Starkey <stark3y@gmail.com>
package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
)

var (
	OpcodeSync       [4]byte = [4]byte{'S', 'Y', 'N', 'C'}
	OpcodeRead       [4]byte = [4]byte{'R', 'E', 'A', 'D'}
	OpcodeCsum       [4]byte = [4]byte{'C', 'S', 'U', 'M'}
	OpcodeCRC        [4]byte = [4]byte{'C', 'R', 'C', 'C'}
	OpcodeErase      [4]byte = [4]byte{'E', 'R', 'A', 'S'}
	OpcodeWrite      [4]byte = [4]byte{'W', 'R', 'I', 'T'}
	OpcodeSeal       [4]byte = [4]byte{'S', 'E', 'A', 'L'}
	OpcodeGo         [4]byte = [4]byte{'G', 'O', 'G', 'O'}
	OpcodeInfo       [4]byte = [4]byte{'I', 'N', 'F', 'O'}
	ResponseSync     [4]byte = [4]byte{'P', 'I', 'C', 'O'}
	ResponseSyncWota [4]byte = [4]byte{'W', 'O', 'T', 'A'}
	ResponseOK       [4]byte = [4]byte{'O', 'K', 'O', 'K'}
	ResponseErr      [4]byte = [4]byte{'E', 'R', 'R', '!'}
)

var ErrNotSynced error = errors.New("not synced")

func readResponse(rw io.ReadWriter, responseLen int) ([]byte, error) {
	buf := make([]byte, responseLen)
	atLeast := len(ResponseErr)

	for total := 0; total < responseLen; {
		n, err := io.ReadAtLeast(rw, buf[total:], atLeast)
		if err != nil {
			return nil, err
		}
		total += n
		atLeast = responseLen - total

		if total >= len(ResponseErr) {
			if bytes.HasPrefix(buf, ResponseErr[:]) {
				return nil, fmt.Errorf("received error response")
			} else if !bytes.HasPrefix(buf, ResponseOK[:]) {
				return nil, fmt.Errorf("received unexpected response")
			}
		}
	}

	return buf, nil
}

type SyncCommand struct {
}

func (c *SyncCommand) Execute(rw io.ReadWriter) error {
	// TODO: Can we do better than an arbitrary 4096 length here?
	// The idea is to just drain whatever is on the port.
	var resp [4096]byte

	n, err := rw.Write(OpcodeSync[:])
	if err != nil {
		return err
	} else if n != len(OpcodeSync) {
		return fmt.Errorf("unexpected write length: %v", n)
	}

	n, err = io.ReadAtLeast(rw, resp[:], len(ResponseSync))
	if err != nil {
		return err
	}

	// Different responses for picowota and rp2040_serial_bootloader
	validSyncResponses := [][4]byte{
		ResponseSync,
		ResponseSyncWota,
	}

	for _, r := range validSyncResponses {
		if bytes.HasSuffix(resp[:n], r[:]) {
			return nil
		}
	}

	return ErrNotSynced
}

type ReadCommand struct {
	Addr uint32
	Len  uint32
	Data []byte
}

func (c *ReadCommand) Execute(rw io.ReadWriter) error {
	// Re-use for command and response.
	buf := make([]byte, len(OpcodeRead)+4+4)

	copy(buf[0:], OpcodeRead[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeRead)+4+4 {
		return fmt.Errorf("unexpected write length: %v", n)
	}

	resp, err := readResponse(rw, len(ResponseOK)+int(c.Len))
	if err != nil {
		return err
	}

	// Slice off the response
	c.Data = resp[len(ResponseOK):]

	return nil
}

type CsumCommand struct {
	Addr uint32
	Len  uint32
	Csum uint32
}

func calculateChecksum(data []byte) uint32 {
	alignedLen := ((len(data) + 3) / 4) * 4
	buf := make([]byte, alignedLen)
	copy(buf, data)

	result := uint32(0)
	for i := 0; i < alignedLen; i += 4 {
		result += binary.LittleEndian.Uint32(buf[i:])
	}

	return result
}

func (c *CsumCommand) Execute(rw io.ReadWriter) error {
	buf := make([]byte, len(OpcodeCsum)+4+4)

	copy(buf[0:], OpcodeCsum[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeCsum)+4+4 {
		return fmt.Errorf("unexpected write length: %v", n)
	}

	resp, err := readResponse(rw, len(ResponseOK)+4)
	if err != nil {
		return err
	}

	c.Csum = binary.LittleEndian.Uint32(resp[4:])

	return nil
}

type CRCCommand struct {
	Addr uint32
	Len  uint32
	CRC  uint32
}

func (c *CRCCommand) Execute(rw io.ReadWriter) error {
	buf := make([]byte, len(OpcodeCRC)+4+4)

	copy(buf[0:], OpcodeCRC[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeCRC)+4+4 {
		return fmt.Errorf("unexpected write length: %v", n)
	}

	resp, err := readResponse(rw, len(ResponseOK)+4)
	if err != nil {
		return err
	}

	c.CRC = binary.LittleEndian.Uint32(resp[4:])

	return nil
}

type EraseCommand struct {
	Addr uint32
	Len  uint32
}

func (c *EraseCommand) Execute(rw io.ReadWriter) error {
	// Re-use for command and response.
	buf := make([]byte, len(OpcodeErase)+4+4)

	copy(buf[0:], OpcodeErase[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeErase)+4+4 {
		return fmt.Errorf("unexpected write length: %v", n)
	}

	_, err = readResponse(rw, len(ResponseOK))
	if err != nil {
		return err
	}

	return nil
}

type WriteCommand struct {
	Addr uint32
	Len  uint32
	Data []byte
}

func (c *WriteCommand) Execute(rw io.ReadWriter) error {
	// Re-use for command and response.
	buf := make([]byte, len(OpcodeWrite)+4+4+len(c.Data))

	copy(buf[0:], OpcodeWrite[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)
	copy(buf[12:], c.Data)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(buf) {
		return fmt.Errorf("unexpected write length: %v", n)
	}

	resp, err := readResponse(rw, len(ResponseOK)+4)
	if err != nil {
		return err
	}

	respCRC := binary.LittleEndian.Uint32(resp[4:])
	calcCRC := crc32.ChecksumIEEE(c.Data)

	if respCRC != calcCRC {
		return fmt.Errorf("CRC mismatch: 0x%08x vs 0x%08x", respCRC, calcCRC)
	}

	return nil
}

type SealCommand struct {
	Addr uint32
	Len  uint32
	CRC  uint32
}

func NewSealCommand(addr uint32, data []byte) *SealCommand {
	return &SealCommand{
		Addr: addr,
		Len:  uint32(len(data)),
		CRC:  crc32.ChecksumIEEE(data),
	}
}

func (c *SealCommand) Execute(rw io.ReadWriter) error {
	buf := make([]byte, len(OpcodeSeal)+4+4+4)

	copy(buf[0:], OpcodeSeal[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)
	binary.LittleEndian.PutUint32(buf[12:], c.CRC)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeSeal)+4+4+4 {
		return fmt.Errorf("unexpected write length: %v", n)
	}

	_, err = readResponse(rw, len(ResponseOK))
	if err != nil {
		return err
	}

	return nil
}

type GoCommand struct {
	Addr uint32
}

func (c *GoCommand) Execute(rw io.ReadWriter) error {
	buf := make([]byte, len(OpcodeGo)+4)

	copy(buf[0:], OpcodeGo[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(buf) {
		return fmt.Errorf("unexpected write length: %v", n)
	}

	// Fire and forget

	return nil
}

type InfoCommand struct {
	FlashAddr  uint32
	FlashSize  uint32
	EraseSize  uint32
	WriteSize  uint32
	MaxDataLen uint32
}

func (c *InfoCommand) Execute(rw io.ReadWriter) error {
	buf := make([]byte, len(OpcodeInfo))

	copy(buf[0:], OpcodeInfo[:])

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeInfo) {
		return fmt.Errorf("unexpected write length: %v", n)
	}

	resp, err := readResponse(rw, len(ResponseOK)+(4*5))
	if err != nil {
		return err
	}

	c.FlashAddr = binary.LittleEndian.Uint32(resp[4:])
	c.FlashSize = binary.LittleEndian.Uint32(resp[8:])
	c.EraseSize = binary.LittleEndian.Uint32(resp[12:])
	c.WriteSize = binary.LittleEndian.Uint32(resp[16:])
	c.MaxDataLen = binary.LittleEndian.Uint32(resp[20:])

	return nil
}
