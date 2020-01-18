package vm

import (
	"encoding/binary"
	"io/ioutil"
	"log"
)

// RAM
const MaxMemorySize uint16 = 65535

const (
	MR_KBSR uint16 = 0xfe00 // keyboard status
	MR_KBDR uint16 = 0xfe02 // keyboard data
)

type CheckKey func() bool
type GetChar func() uint16

type LC3RAM struct {
	CheckKey
	GetChar
	Storage [MaxMemorySize]uint16
}

func (m *LC3RAM) Write(address, val uint16) {
	m.Storage[address] = val
}

func (m *LC3RAM) Read(address uint16) uint16 {
	if address == MR_KBSR {
		if m.CheckKey() {
			m.Storage[MR_KBSR] = 1 << 15
			// read a single ASCII char
			m.Storage[MR_KBDR] = m.GetChar()
		} else {
			m.Storage[MR_KBSR] = 0
		}
	}
	return m.Storage[address]
}

func (m *LC3RAM) Load(path string) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Can't read file, reason: %v", err)
	}

	origin := binary.BigEndian.Uint16(b[:2])

	for i := 2; i < len(b); i += 2 {
		m.Storage[origin] = binary.BigEndian.Uint16(b[i : i+2])
		origin++
	}
}