// SPDX-License-Identifier: MIT
// Copyright (c) 2021 Brian Starkey <stark3y@gmail.com>
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	tty "github.com/jacobsa/go-serial/serial"
)

var (
	OpcodeSync   [4]byte = [4]byte{ 'P', 'I', 'C', 'O' }
	ResponseSync [4]byte = [4]byte{ 'S', 'Y', 'N', 'C' }
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

	// TODO: Save Cancel somewhere
	ctx, _ := context.WithCancel(context.Background())

	tick := time.NewTicker(300 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <- tick.C:
			port.Write([]byte("foolio"))
		default:
			var sc SyncCommand
			err = (&sc).Execute(port)
			if err != nil {
				fmt.Println("sync:", err)
			} else {
				fmt.Println("Synced!")
			}
			time.Sleep(1 * time.Second)
		}
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
