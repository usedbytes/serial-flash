// SPDX-License-Identifier: MIT
// Copyright (c) 2021 Brian Starkey <stark3y@gmail.com>
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	tty "github.com/jacobsa/go-serial/serial"
)

var (
	OpcodeSync   [4]byte = [4]byte{ 'S', 'Y', 'N', 'C' }
	OpcodeRead   [4]byte = [4]byte{ 'R', 'E', 'A', 'D' }
	OpcodeCsum   [4]byte = [4]byte{ 'C', 'S', 'U', 'M' }
	ResponseSync [4]byte = [4]byte{ 'P', 'I', 'C', 'O' }
	ResponseOK   [4]byte = [4]byte{ 'O', 'K', 'O', 'K' }
	ResponseErr  [4]byte = [4]byte{ 'E', 'R', 'R', '!' }
)

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
		return fmt.Errorf("unexpectead write length: %v", n)
	}

	n, err = rw.Read(resp[:])
	if err != nil {
		return err
	}

	if !bytes.HasSuffix(resp[:n], ResponseSync[:]) {
		return fmt.Errorf("not synced")
	}

	return nil
}

type ReadCommand struct {
	Addr uint32
	Len  uint32
	Data []byte
}

func (c *ReadCommand) Execute(rw io.ReadWriter) error {
	// Re-use for command and response.
	buf := make([]byte, len(OpcodeRead) + 4 + 4, len(OpcodeRead) + int(c.Len))

	copy(buf[0:], OpcodeRead[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeRead) + 4 + 4 {
		return fmt.Errorf("unexpectead write length: %v", n)
	}

	// Re-slice to full size
	buf = buf[:cap(buf)]

	n, err = rw.Read(buf)
	if err != nil {
		return err
	}

	if !bytes.HasPrefix(buf, ResponseOK[:]) {
		return fmt.Errorf("received error response")
	}

	// Slice off the response
	c.Data = buf[len(ResponseOK):]

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
	for i := 0; i < alignedLen; i+=4 {
		result += binary.LittleEndian.Uint32(buf[i:])
	}

	return result
}

func (c *CsumCommand) Execute(rw io.ReadWriter) error {
	// Re-use for command and response.
	buf := make([]byte, len(OpcodeCsum) + 4 + 4)

	copy(buf[0:], OpcodeCsum[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeCsum) + 4 + 4 {
		return fmt.Errorf("unexpectead write length: %v", n)
	}

	// Re-slice to single arg
	buf = buf[:len(ResponseOK) + 4]

	n, err = rw.Read(buf)
	if err != nil {
		return err
	}

	if !bytes.HasPrefix(buf, ResponseOK[:]) {
		return fmt.Errorf("received error response")
	}

	c.Csum = binary.LittleEndian.Uint32(buf[4:])

	return nil
}

func run() error {
	options := tty.OpenOptions{
		PortName: "/dev/ttyUSB0",
		BaudRate: 115200,
		DataBits: 8,
		StopBits: 1,
		MinimumReadSize: 1,
		InterCharacterTimeout: 100,
	}

	port, err := tty.Open(options)
	if err != nil {
		return fmt.Errorf("tty.Open: %v", err)
	}
	defer port.Close()

	// Try and sync
	for i := 0; i < 5; i++ {
		var sc SyncCommand
		err = (&sc).Execute(port)
		if err != nil {
			fmt.Println("sync:", err)
		} else {
			fmt.Println("Synced!")
			break
		}
	}

	// Bail out
	if err != nil {
		return err
	}

	rc := &ReadCommand{
		Addr: 0x10000000,
		Len:  240,
	}

	err = rc.Execute(port)
	if err != nil {
		return err
	}

	fmt.Printf("0x%8x, %d bytes\n", rc.Addr, rc.Len)

	fmt.Println(hex.Dump(rc.Data))
	fmt.Printf("Calc CSUM: 0x%08x\n", calculateChecksum(rc.Data))

	cc := &CsumCommand{
		Addr: rc.Addr,
		Len:  rc.Len,
	}

	err = cc.Execute(port)
	if err != nil {
		return err
	}

	fmt.Printf("Returned CSUM: 0x%08x\n", cc.Csum)

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
