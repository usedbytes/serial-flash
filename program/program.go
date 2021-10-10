// SPDX-License-Identifier: MIT
// Copyright (c) 2021 Brian Starkey <stark3y@gmail.com>
package program

import (
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

func sync(rw io.ReadWriter) error {
	var err error

	for i := 0; i < maxSyncAttempts; i++ {
		var sc protocol.SyncCommand
		err = (&sc).Execute(rw)
		if err == nil {
			return nil
		}
	}

	return err
}

func align(val, to uint32) uint32 {
	return (val + (to - 1)) & ^(to - 1)
}

func Program(rw io.ReadWriter, img *Image) error {
	err := sync(rw)
	if err != nil {
		return fmt.Errorf("sync: %v", err)
	}

	ic := &protocol.InfoCommand{ }
	err = ic.Execute(rw)
	if err != nil {
		return fmt.Errorf("info: %v", err)
	}

	pad := align(uint32(len(img.Data)), ic.WriteSize) - uint32(len(img.Data))
	data := append(img.Data, make([]byte, pad)...)

	if (img.Addr < ic.FlashAddr) || (img.Addr + uint32(len(data)) > ic.FlashAddr + ic.FlashSize) {
		return fmt.Errorf("image of %d bytes doesn't fit in flash at 0x%08x", len(data), img.Addr)
	}

	ec := &protocol.EraseCommand{
		Addr: img.Addr,
		Len: align(uint32(len(data)), ic.EraseSize),
	}

	err = ec.Execute(rw)
	if err != nil {
		return fmt.Errorf("erase: %v", err)
	}

	//numChunks := (len(data) + (ic.MaxDataLen - 1)) / ic.MaxDataLen

	for start := uint32(0); start < uint32(len(data)); start += ic.MaxDataLen {
		end := start + ic.MaxDataLen
		if end > uint32(len(data)) {
			end = uint32(len(data))
		}

		wc := &protocol.WriteCommand{
			Addr: img.Addr + start,
			Len: end - start,
			Data: data[start:end],
		}
		err = wc.Execute(rw)
		if err != nil {
			return fmt.Errorf("write: %v", err)
		}
	}

	sc := protocol.NewSealCommand(img.Addr, data)
	err = sc.Execute(rw)
	if err != nil {
		return fmt.Errorf("seal: %v", err)
	}

	return nil
}
