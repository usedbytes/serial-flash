package program

import (
	"debug/elf"
	"sort"
)

const (
	FlashBase uint64 = 0x10000000
	FlashSize uint64 = (2 * 1024 * 1024)
)

type InFlashFunc func(addr, size uint64) bool

func DefaultInFlashFunc(addr, size uint64) bool {
	return (addr >= FlashBase) && (addr+size <= FlashBase+FlashSize)
}

type chunk struct {
	PAddr uint64
	Data  []byte
}

type byPAddr []*chunk

func (p byPAddr) Len() int           { return len(p) }
func (p byPAddr) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p byPAddr) Less(i, j int) bool { return p[i].PAddr < p[j].PAddr }

func inProg(vaddr, size uint64, prog *elf.Prog) bool {
	return (vaddr >= prog.Vaddr) && (vaddr+size <= (prog.Vaddr + prog.Memsz))
}

func LoadELF(fname string, inFlash InFlashFunc) (*Image, error) {
	f, err := elf.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	chunks := []*chunk{}

	for _, prog := range f.Progs {
		if !inFlash(prog.Paddr, prog.Memsz) {
			continue
		}

		for _, sec := range f.Sections {
			if sec.Size > 0 && inProg(sec.Addr, sec.Size, prog) {
				progOffset := sec.Addr - prog.Vaddr
				data, err := sec.Data()
				if err != nil {
					return nil, err
				}

				chunk := &chunk{
					PAddr: prog.Paddr + progOffset,
					Data:  data,
				}
				chunks = append(chunks, chunk)
			}
		}
	}

	sort.Sort(byPAddr(chunks))

	minPAddr := chunks[0].PAddr
	maxPAddr := chunks[len(chunks)-1].PAddr + uint64(len(chunks[len(chunks)-1].Data))

	data := make([]byte, maxPAddr-minPAddr)

	for _, c := range chunks {
		copy(data[c.PAddr-minPAddr:], c.Data)
	}

	return &Image{
		Addr: uint32(minPAddr),
		Data: data,
	}, nil
}
