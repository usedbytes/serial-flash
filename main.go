// SPDX-License-Identifier: MIT
// Copyright (c) 2021 Brian Starkey <stark3y@gmail.com>
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	tty "github.com/jacobsa/go-serial/serial"

	protocol "github.com/usedbytes/serial-flash/protocol"
)

func align(val, to uint32) uint32 {
	return (val + (to - 1)) & ^(to - 1)
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("Usage: %s PORT", os.Args[0])
	}

	if len(os.Args) < 4 {
		return fmt.Errorf("Usage: %s PORT BINARY ADDR", os.Args[0])
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

	fname := os.Args[2]
	str_addr := os.Args[3]

	f, err := os.Open(fname)
	if err != nil {
		return err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	// Try and sync
	for i := 0; i < 5; i++ {
		var sc protocol.SyncCommand
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

	ic := &protocol.InfoCommand{ }

	fmt.Println("Get info...");
	err = ic.Execute(rw)
	if err != nil {
		return err
	}
	fmt.Printf("FlashAddr:  0x%08x\n", ic.FlashAddr)
	fmt.Printf("FlashSize:  0x%08x\n", ic.FlashSize)
	fmt.Printf("EraseSize:  0x%08x\n", ic.EraseSize)
	fmt.Printf("WriteSize:  0x%08x\n", ic.WriteSize)
	fmt.Printf("MaxDataLen: 0x%08x\n", ic.MaxDataLen)

	pad := align(uint32(len(data)), ic.WriteSize) - uint32(len(data))
	data = append(data, make([]byte, pad)...)

	addr64, err := strconv.ParseUint(str_addr, 0, 32)
	if err != nil {
		return err
	}

	addr := uint32(addr64)

	if (addr < ic.FlashAddr) || (addr + uint32(len(data)) > ic.FlashAddr + ic.FlashSize) {
		return fmt.Errorf("image of %d bytes doesn't fit in flash at 0x%08x", len(data), uint32(addr))
	}

	ec := &protocol.EraseCommand{
		Addr: addr,
		Len: align(uint32(len(data)), ic.EraseSize),
	}

	fmt.Println("Erasing...");
	err = ec.Execute(rw)
	fmt.Println("Done...");
	if err != nil {
		return err
	}

	chunkLen := uint32(ic.MaxDataLen)
	for start := uint32(0); start < uint32(len(data)); start += chunkLen {
		end := start + chunkLen
		if end > uint32(len(data)) {
			end = uint32(len(data))
		}

		wc := &protocol.WriteCommand{
			Addr: addr + start,
			Len: end - start,
			Data: data[start:end],
		}
		fmt.Printf("Writing %d bytes to 0x%08x\n", wc.Len, wc.Addr);
		err = wc.Execute(rw)
		fmt.Println("Done...");
		if err != nil {
			return err
		}
	}

	sc := protocol.NewSealCommand(addr, data)

	fmt.Println("Sealing...")
	err = sc.Execute(rw)
	if err != nil {
		return err
	}

	gc := &protocol.GoCommand{
		Addr: addr,
	}

	fmt.Println("Jumping...")
	err = gc.Execute(rw)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
