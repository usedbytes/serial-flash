package program

import (
	"io"
	"os"
)

func LoadBin(fname string, base uint32) (*Image, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(f)

	return &Image{
		Addr: base,
		Data: data,
	}, nil
}
