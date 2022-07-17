// SPDX-License-Identifier: MIT
// Copyright (c) 2021 Brian Starkey <stark3y@gmail.com>
package program

import (
	"errors"
	"fmt"
	"io"

	"github.com/usedbytes/serial-flash/protocol"
)

const (
	maxSyncAttempts int = 5
)

type Image struct {
	Addr uint32
	Data []byte
}

type ProgressReport struct {
	Stage    string
	Progress int
	Max      int
}

func reportProgress(reportChan chan<- ProgressReport, stage string, progress int, max int) {
	if reportChan == nil {
		return
	}

	reportChan <- ProgressReport{
		Stage:    stage,
		Progress: progress,
		Max:      max,
	}
}

func sync(rw io.ReadWriter, progress chan<- ProgressReport) error {
	var err error

	for i := 0; i < maxSyncAttempts; i++ {
		reportProgress(progress, "Synchronising", i, maxSyncAttempts)

		var sc protocol.SyncCommand
		err = (&sc).Execute(rw)

		reportProgress(progress, "Synchronising", i+1, maxSyncAttempts)
		if err == nil {
			return nil
		} else if !errors.Is(err, protocol.ErrNotSynced) {
			return err
		}
	}

	return err
}

func align(val, to uint32) uint32 {
	return (val + (to - 1)) & ^(to - 1)
}

func Program(rw io.ReadWriter, img *Image, progress chan<- ProgressReport) error {
	if progress != nil {
		defer close(progress)
	}

	err := sync(rw, progress)
	if err != nil {
		return fmt.Errorf("sync: %v", err)
	}

	reportProgress(progress, "Querying device info", 0, 1)
	ic := &protocol.InfoCommand{}
	err = ic.Execute(rw)
	reportProgress(progress, "Querying device info", 1, 1)
	if err != nil {
		return fmt.Errorf("info: %v", err)
	}

	pad := align(uint32(len(img.Data)), ic.WriteSize) - uint32(len(img.Data))
	data := append(img.Data, make([]byte, pad)...)

	if img.Addr < ic.FlashAddr {
		return fmt.Errorf("image load address too low: 0x%08x < 0x%08x", img.Addr, ic.FlashAddr)
	}

	if img.Addr+uint32(len(data)) > ic.FlashAddr+ic.FlashSize {
		return fmt.Errorf("image of %d bytes doesn't fit in flash at 0x%08x", len(data), img.Addr)
	}

	// The protocol allows for larger erasures, but this way each individual
	// command takes less time, meaning transports don't time out
	eraseLen := int(align(uint32(len(data)), ic.EraseSize))
	reportProgress(progress, "Erasing", 0, eraseLen)
	for start := uint32(0); start < uint32(eraseLen); start += ic.EraseSize {
		end := start + ic.EraseSize

		ec := &protocol.EraseCommand{
			Addr: img.Addr + start,
			Len:  ic.EraseSize,
		}
		err = ec.Execute(rw)
		reportProgress(progress, "Erasing", int(end), eraseLen)
		if err != nil {
			return fmt.Errorf("erase: %v", err)
		}
	}

	reportProgress(progress, "Writing", 0, len(data))
	for start := uint32(0); start < uint32(len(data)); start += ic.MaxDataLen {
		end := start + ic.MaxDataLen
		if end > uint32(len(data)) {
			end = uint32(len(data))
		}

		wc := &protocol.WriteCommand{
			Addr: img.Addr + start,
			Len:  end - start,
			Data: data[start:end],
		}
		err = wc.Execute(rw)
		reportProgress(progress, "Writing", int(end), len(data))
		if err != nil {
			return fmt.Errorf("write: %v", err)
		}
	}

	reportProgress(progress, "Finalising", 0, 1)
	sc := protocol.NewSealCommand(img.Addr, data)
	err = sc.Execute(rw)
	reportProgress(progress, "Finalising", 1, 1)
	if err != nil {
		return fmt.Errorf("seal: %v", err)
	}

	return nil
}
