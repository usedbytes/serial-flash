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

	"github.com/usedbytes/serial-flash/protocol"
	"github.com/usedbytes/serial-flash/program"
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

	addr64, err := strconv.ParseUint(str_addr, 0, 32)
	if err != nil {
		return fmt.Errorf("parsing address %v: %v", str_addr, err)
	}

	f, err := os.Open(fname)
	if err != nil {
		return err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	img := &program.Image{
		Addr: uint32(addr64),
		Data: data,
	}

	prog := make(chan program.ProgressReport)

	go func() {
		for p := range prog {
			fmt.Println(p.Stage, p.Progress, p.Max)
		}
	}()

	err = program.Program(rw, img, prog)
	if err != nil {
		return err
	}

	gc := &protocol.GoCommand{
		Addr: img.Addr,
	}

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
