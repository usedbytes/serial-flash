// SPDX-License-Identifier: MIT
// Copyright (c) 2021 Brian Starkey <stark3y@gmail.com>
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cheggaaa/pb"
	tty "github.com/jacobsa/go-serial/serial"

	"github.com/usedbytes/serial-flash/program"
	"github.com/usedbytes/serial-flash/protocol"
)

func align(val, to uint32) uint32 {
	return (val + (to - 1)) & ^(to - 1)
}

func usage() error {
	return fmt.Errorf("Usage: %s PORT FILE [BASE]", os.Args[0])
}

func run() error {
	if len(os.Args) < 3 {
		return usage()
	}

	var img *program.Image
	var err error

	fname := os.Args[2]
	if filepath.Ext(fname) == ".elf" {
		if len(os.Args) > 3 {
			fmt.Println("base address can't be specified for ELF files")
			return usage()
		}

		img, err = program.LoadELF(fname, program.DefaultInFlashFunc)
		if err != nil {
			return fmt.Errorf("loading ELF %v: %v", fname, err)
		}
	} else {
		if len(os.Args) < 4 {
			fmt.Println("base address mustt be specified for binary files")
			return usage()
		}

		addr64, err := strconv.ParseUint(os.Args[3], 0, 32)
		if err != nil {
			return fmt.Errorf("parsing address %v: %v", os.Args[3], err)
		}

		img, err = program.LoadBin(fname, uint32(addr64))
		if err != nil {
			return fmt.Errorf("loading binary %v: %v", fname, err)
		}
	}

	var rw io.ReadWriter

	port := os.Args[1]
	if strings.HasPrefix(port, "tcp:") {
		conn, err := net.Dial("tcp", port[len("tcp:"):])
		if err != nil {
			return fmt.Errorf("net.Dial %s: %v", port[len("tcp:"):], err)
		}
		defer conn.Close()

		fmt.Println("Opened connection to", port[len("tcp:"):])

		// FIXME: On Pico-W a packet sent immediately after opening
		// the connection seems to get lost.
		time.Sleep(1 * time.Second)

		rw = conn
	} else {
		options := tty.OpenOptions{
			PortName:              port,
			BaudRate:              921600,
			DataBits:              8,
			StopBits:              1,
			MinimumReadSize:       1,
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

	prog := make(chan program.ProgressReport)
	done := make(chan bool)

	go func() {
		var last program.ProgressReport
		var bar *pb.ProgressBar

		for p := range prog {
			if p.Stage != last.Stage {
				if bar != nil {
					bar.Finish()
				}

				fmt.Println(p.Stage + ":")
				bar = pb.New(p.Max)
				bar.ShowSpeed = true
				bar.SetMaxWidth(80)
				//bar.Prefix(p.Stage)
				bar.Start()
			}

			bar.Set(p.Progress)
			bar.Update()
			last = p
		}

		if bar != nil {
			bar.Finish()
		}

		done <- true
	}()

	err = program.Program(rw, img, prog)
	<-done

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
