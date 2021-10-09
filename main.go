// SPDX-License-Identifier: MIT
// Copyright (c) 2021 Brian Starkey <stark3y@gmail.com>
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	tty "github.com/jacobsa/go-serial/serial"
)

var (
	OpcodeSync    [4]byte = [4]byte{ 'S', 'Y', 'N', 'C' }
	OpcodeRead    [4]byte = [4]byte{ 'R', 'E', 'A', 'D' }
	OpcodeCsum    [4]byte = [4]byte{ 'C', 'S', 'U', 'M' }
	OpcodeCRC     [4]byte = [4]byte{ 'C', 'R', 'C', 'C' }
	OpcodeErase   [4]byte = [4]byte{ 'E', 'R', 'A', 'S' }
	OpcodeWrite   [4]byte = [4]byte{ 'W', 'R', 'I', 'T' }
	ResponseSync  [4]byte = [4]byte{ 'P', 'I', 'C', 'O' }
	ResponseOK    [4]byte = [4]byte{ 'O', 'K', 'O', 'K' }
	ResponseErr   [4]byte = [4]byte{ 'E', 'R', 'R', '!' }
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

	n, err = io.ReadAtLeast(rw, resp[:], len(ResponseSync))
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

	n, err = io.ReadFull(rw, buf)
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

	n, err = io.ReadFull(rw, buf)
	if err != nil {
		return err
	}

	if !bytes.HasPrefix(buf, ResponseOK[:]) {
		return fmt.Errorf("received error response")
	}

	c.Csum = binary.LittleEndian.Uint32(buf[4:])

	return nil
}

type CRCCommand struct {
	Addr uint32
	Len  uint32
	CRC uint32
}

func (c *CRCCommand) Execute(rw io.ReadWriter) error {
	// Re-use for command and response.
	buf := make([]byte, len(OpcodeCRC) + 4 + 4)

	copy(buf[0:], OpcodeCRC[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeCRC) + 4 + 4 {
		return fmt.Errorf("unexpectead write length: %v", n)
	}

	// Re-slice to single arg
	buf = buf[:len(ResponseOK) + 4]

	n, err = io.ReadFull(rw, buf)
	if err != nil {
		return err
	}

	if !bytes.HasPrefix(buf, ResponseOK[:]) {
		return fmt.Errorf("received error response")
	}

	c.CRC = binary.LittleEndian.Uint32(buf[4:])

	return nil
}

type EraseCommand struct {
	Addr uint32
	Len  uint32
}

func (c *EraseCommand) Execute(rw io.ReadWriter) error {
	// Re-use for command and response.
	buf := make([]byte, len(OpcodeErase) + 4 + 4)

	copy(buf[0:], OpcodeErase[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(OpcodeErase) + 4 + 4 {
		return fmt.Errorf("unexpectead write length: %v", n)
	}

	n, err = io.ReadAtLeast(rw, buf[:], len(ResponseOK))
	if err != nil {
		return err
	}

	if !bytes.HasPrefix(buf, ResponseOK[:]) {
		return fmt.Errorf("received error response")
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
	buf := make([]byte, len(OpcodeWrite) + 4 + 4 + len(c.Data))

	copy(buf[0:], OpcodeWrite[:])
	binary.LittleEndian.PutUint32(buf[4:], c.Addr)
	binary.LittleEndian.PutUint32(buf[8:], c.Len)
	copy(buf[12:], c.Data)

	n, err := rw.Write(buf)
	if err != nil {
		return err
	} else if n != len(buf) {
		return fmt.Errorf("unexpectead write length: %v", n)
	}

	// Re-slice to single response arg
	buf = buf[:len(ResponseOK) + 4]

	n, err = io.ReadFull(rw, buf)
	if err != nil {
		return err
	}

	if !bytes.HasPrefix(buf, ResponseOK[:]) {
		return fmt.Errorf("received error response")
	}

	response_crc := binary.LittleEndian.Uint32(buf[4:])
	calc_crc := crc32.ChecksumIEEE(c.Data)

	if response_crc != calc_crc {
		return fmt.Errorf("CRC mismatch: 0x%08x vs 0x%08x", response_crc, calc_crc)
	}

	return nil
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("Usage: %s PORT", os.Args[0])
	}

	var rw io.ReadWriter
	var err error

	port := os.Args[1]
	if strings.HasPrefix(port, "tcp:") {
		conn, err := net.Dial("tcp", port[len("tcp:"):])
		if err != nil {
			return fmt.Errorf("net.Dial %s: %v", port[len("tcp:"):], err)
		}
		defer conn.Close()

		fmt.Println("Opened connection to", port[len("tcp:"):])

		rw = conn
	} else {
		options := tty.OpenOptions{
			PortName: port,
			BaudRate: 115200,
			DataBits: 8,
			StopBits: 1,
			MinimumReadSize: 1,
			InterCharacterTimeout: 100,
		}

		ser, err := tty.Open(options)
		if err != nil {
			return fmt.Errorf("tty.Open %s: %v", port, err)
		}
		defer ser.Close()

		fmt.Println("Opened", port)

		rw = ser
	}

	// Try and sync
	for i := 0; i < 5; i++ {
		var sc SyncCommand
		fmt.Println("sync", i)
		err = (&sc).Execute(rw)
		if err != nil {
			fmt.Println("sync:", err)
		} else {
			fmt.Println("Synced!")
			break
		}
	}

	// Sync failed. Bail out
	if err != nil {
		return err
	}

	rc := &ReadCommand{
		Addr: 0x10000000 + (1024 * 1024),
		Len:  4096,
	}

	ec := &EraseCommand{
		Addr: rc.Addr,
		Len: rc.Len,
	}

	wc := &WriteCommand{
		Addr: ec.Addr,
		Len: rc.Len,
		Data: make([]byte, rc.Len),
	}


	rand.Seed(time.Now().UnixNano())
	n, err := rand.Read(wc.Data)
	if n != len(wc.Data) {
		return fmt.Errorf("unexpected random len %v", n)
	} else if err != nil {
		return err
	}

	cc := &CsumCommand{
		Addr: rc.Addr,
		Len:  rc.Len,
	}

	cr := &CRCCommand{
		Addr: rc.Addr,
		Len:  rc.Len,
	}

	fmt.Println("Erasing...");
	err = ec.Execute(rw)
	fmt.Println("Done...");
	if err != nil {
		return err
	}

	fmt.Println("Writing...");
	err = wc.Execute(rw)
	fmt.Println("Done...");
	if err != nil {
		return err
	}

	fmt.Println("Reading...");
	err = rc.Execute(rw)
	fmt.Println("Done...");
	if err != nil {
		return err
	}

	fmt.Printf("0x%8x, %d bytes\n", rc.Addr, rc.Len)

	fmt.Println(hex.Dump(rc.Data))
	fmt.Printf("Calc CSUM: 0x%08x\n", calculateChecksum(rc.Data))
	fmt.Printf("Calc CRC:  0x%08x\n", crc32.ChecksumIEEE(rc.Data))

	err = cc.Execute(rw)
	if err != nil {
		return err
	}

	fmt.Printf("Resp CSUM: 0x%08x\n", cc.Csum)

	err = cr.Execute(rw)
	if err != nil {
		return err
	}

	fmt.Printf("Resp CRC:  0x%08x\n", cr.CRC)

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
