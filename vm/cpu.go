package vm

import (
	"fmt"
	"io"
	"log"
)

// Registers
const (
	R_R0 uint16 = iota
	R_R1
	R_R2
	R_R3
	R_R4
	R_R5
	R_R6
	R_R7
	R_PC // program counter
	R_COND
	R_COUNT
)

// Opcodes
const (
	OP_BR   uint16 = iota // branch
	OP_ADD                // add
	OP_LD                 // load
	OP_ST                 // store
	OP_JSR                // jump register
	OP_AND                // bitwise and
	OP_LDR                // load register
	OP_STR                // store register
	OP_RTI                // unused
	OP_NOT                // bitwise not
	OP_LDI                // load indirect
	OP_STI                // store indirect
	OP_JMP                // jump
	OP_RES                // reserved (unused)
	OP_LEA                // load effective address
	OP_TRAP               // execute trap
)

// Condition Flags
const (
	FL_POS uint16 = 1 << 0 // Positive
	FL_ZRO uint16 = 1 << 1 // Zero
	FL_NEG uint16 = 1 << 2 // Negative
)

const PC_START uint16 = 0x3000

// LC3CPU describes CPU abstraction.
type LC3CPU struct {
	registers          [R_COUNT]uint16
	RAM                *LC3RAM
	currentInstruction uint16
	currentOperation   uint16
	isRunning          bool
	StartPosition      uint16
	output             io.Writer
}

// NewCPU creates new LC-3 CPU instance.
func NewCPU(ram *LC3RAM, output io.Writer) *LC3CPU {
	return &LC3CPU{
		StartPosition: PC_START,
		RAM:           ram,
		output:        output,
	}
}

// Reset resets CPU to initial state.
func (v *LC3CPU) Reset() {
	v.registers = [R_COUNT]uint16{}
	v.RAM = &LC3RAM{
		CheckKey: CheckKeyPressed,
		GetChar:  GetCharFromStdin,
	}
	v.currentInstruction = 0
	v.currentOperation = 0
	v.isRunning = false
}

// Run runs CPU.
func (v *LC3CPU) Run() {
	// Set the PC to starting position
	// 0x3000 is the default
	v.registers[R_PC] = v.StartPosition
	v.isRunning = true
	for v.isRunning {
		// Fetch
		v.currentInstruction = v.RAM.Read(v.registers[R_PC])
		if v.registers[R_PC] < MaxMemorySize {
			v.registers[R_PC]++
		}
		v.currentOperation = v.currentInstruction >> 12

		switch v.currentOperation {
		case OP_ADD:
			v.add()
		case OP_AND:
			v.and()
		case OP_NOT:
			v.not()
		case OP_BR:
			v.branch()
		case OP_JMP:
			v.jump()
		case OP_JSR:
			v.jumpRegister()
		case OP_LD:
			v.load()
		case OP_LDI:
			v.ldi()
		case OP_LDR:
			v.loadRegister()
		case OP_LEA:
			v.loadEffectiveAddress()
		case OP_ST:
			v.store()
		case OP_STI:
			v.storeIndirect()
		case OP_STR:
			v.storeRegister()
		case OP_TRAP:
			switch v.currentInstruction & 0xFF {
			case TRAP_GETC:
				v.trapGetc()
			case TRAP_OUT:
				v.trapOut()
			case TRAP_PUTS:
				v.trapPuts()
			case TRAP_IN:
				v.trapIn()
			case TRAP_PUTSP:
				v.trapPutsp()
			case TRAP_HALT:
				v.trapHalt()
			}
		case OP_RES:
		case OP_RTI:
		default:
			log.Printf("BAD OPCODE: %016b\n", v.currentOperation)
			v.isRunning = false
		}
	}
}

func (v *LC3CPU) updateFlags(r uint16) {
	if v.registers[r] == 0 {
		v.registers[R_COND] = FL_ZRO
	} else if v.registers[r]>>15 == uint16(1) { //* a 1 in the left-most bit indicates negative */
		v.registers[R_COND] = FL_NEG
	} else {
		v.registers[R_COND] = FL_POS
	}
}

// -------------- Instruction Implementations --------------------

func (v *LC3CPU) add() {
	// destination register (DR)
	r0 := (v.currentInstruction >> 9) & 0x7
	// first operand (SR1)
	r1 := (v.currentInstruction >> 6) & 0x7
	// whether we are in immediate mode
	immFlag := (v.currentInstruction >> 5) & 0x1

	if immFlag == 0x1 {
		imm5 := signExtend(v.currentInstruction&0x1F, 5)
		v.registers[r0] = v.registers[r1] + imm5
	} else {
		r2 := v.currentInstruction & 0x7
		v.registers[r0] = v.registers[r1] + v.registers[r2]
	}

	v.updateFlags(r0)
}

func (v *LC3CPU) and() {
	r0 := (v.currentInstruction >> 9) & 0x7
	r1 := (v.currentInstruction >> 6) & 0x7
	immFlag := (v.currentInstruction >> 5) & 0x1

	if immFlag == 0x1 {
		imm5 := signExtend(v.currentInstruction&0x1F, 5)
		v.registers[r0] = v.registers[r1] & imm5
	} else {
		r2 := v.currentInstruction & 0x7
		v.registers[r0] = v.registers[r1] & v.registers[r2]
	}
	v.updateFlags(r0)
}

func (v *LC3CPU) not() {
	r0 := (v.currentInstruction >> 9) & 0x7
	r1 := (v.currentInstruction >> 6) & 0x7

	v.registers[r0] = ^v.registers[r1]
	v.updateFlags(r0)
}

func (v *LC3CPU) branch() {
	pcOffset := signExtend((v.currentInstruction)&0x1ff, 9)
	condFlag := (v.currentInstruction >> 9) & 0x7
	if (condFlag & v.registers[R_COND]) != 0 { // true
		v.registers[R_PC] += pcOffset
	}
}

func (v *LC3CPU) jump() {
	/* Also handles RET */
	r1 := (v.currentInstruction >> 6) & 0x7
	v.registers[R_PC] = v.registers[r1]
}

func (v *LC3CPU) jumpRegister() {
	r1 := (v.currentInstruction >> 6) & 0x7
	longPcOffset := signExtend(v.currentInstruction&0x7ff, 11)
	longFlag := (v.currentInstruction >> 11) & 1

	v.registers[R_R7] = v.registers[R_PC]
	if longFlag == 1 {
		v.registers[R_PC] += longPcOffset /* JSR */
	} else {
		v.registers[R_PC] = v.registers[r1] /* JSRR */
	}
}

func (v *LC3CPU) load() {
	r0 := (v.currentInstruction >> 9) & 0x7
	pcOffset := signExtend(v.currentInstruction&0x1ff, 9)
	v.registers[r0] = v.RAM.Read(v.registers[R_PC] + pcOffset)
	v.updateFlags(r0)
}

func (v *LC3CPU) ldi() {
	/* destination register (DR) */
	r0 := (v.currentInstruction >> 9) & 0x7
	/* PCoffset 9*/
	pcOffset := signExtend(v.currentInstruction&0x1ff, 9)
	/* add pcOffset to the current PC, look at that RAM location to get the final address */
	v.registers[r0] = v.RAM.Read(v.RAM.Read(v.registers[R_PC] + pcOffset))
	v.updateFlags(r0)
}

func (v *LC3CPU) loadRegister() {
	r0 := (v.currentInstruction >> 9) & 0x7
	r1 := (v.currentInstruction >> 6) & 0x7
	offset := signExtend(v.currentInstruction&0x3F, 6)
	v.registers[r0] = v.RAM.Read(v.registers[r1] + offset)
	v.updateFlags(r0)
}

func (v *LC3CPU) loadEffectiveAddress() {
	r0 := (v.currentInstruction >> 9) & 0x7
	pcOffset := signExtend(v.currentInstruction&0x1ff, 9)
	v.registers[r0] = v.registers[R_PC] + pcOffset
	v.updateFlags(r0)
}

func (v *LC3CPU) store() {
	r0 := (v.currentInstruction >> 9) & 0x7
	pcOffset := signExtend(v.currentInstruction&0x1ff, 9)
	v.RAM.Write(v.registers[R_PC]+pcOffset, v.registers[r0])
}

func (v *LC3CPU) storeIndirect() {
	r0 := (v.currentInstruction >> 9) & 0x7
	pcOffset := signExtend(v.currentInstruction&0x1ff, 9)
	v.RAM.Write(v.RAM.Read(v.registers[R_PC]+pcOffset), v.registers[r0])
}

func (v *LC3CPU) storeRegister() {
	r0 := (v.currentInstruction >> 9) & 0x7
	r1 := (v.currentInstruction >> 6) & 0x7
	offset := signExtend(v.currentInstruction&0x3F, 6)
	v.RAM.Write(v.registers[r1]+offset, v.registers[r0])
}

const (
	TRAP_GETC  = 0x20 // get character from keyboard, not echoed onto the terminal
	TRAP_OUT   = 0x21 // output a character
	TRAP_PUTS  = 0x22 // output a word string
	TRAP_IN    = 0x23 // get character from keyboard, echoed onto the terminal
	TRAP_PUTSP = 0x24 // output a byte string
	TRAP_HALT  = 0x25 // halt the program
)

func (v *LC3CPU) trapGetc() {
	// read a single ASCII char
	v.registers[R_R0] = v.RAM.GetChar()
}

func (v *LC3CPU) trapOut() {
	if _, err := fmt.Fprintf(v.output, "%c", v.registers[R_R0]); err != nil {
		log.Fatalf("Can't write to device: %#v", v.output)
	}
}

func (v *LC3CPU) trapPuts() {
	for i := v.registers[R_R0]; v.RAM.Storage[i] != 0x0000; i++ {
		if _, err := fmt.Fprintf(v.output, "%c", v.RAM.Storage[i]); err != nil {
			log.Fatalf("Can't write to device: %#v", v.output)
		}
	}
}

func (v *LC3CPU) trapIn() {
	if _, err := fmt.Fprintf(v.output, "Input a character: "); err != nil {
		log.Fatalf("Can't write to device: %#v", v.output)
	}

	c := v.RAM.GetChar()

	if _, err := fmt.Fprintf(v.output, "%c", c); err != nil {
		log.Fatalf("Can't write to device: %#v", v.output)
	}

	v.registers[R_R0] = c
}

func (v *LC3CPU) trapPutsp() {
	// one char per byte (two bytes per word)
	// here we need to swap back to
	// big endian format
	for i := v.registers[R_R0]; v.RAM.Storage[i] > 0; i++ {
		ch1 := v.RAM.Storage[i] & 0xFF
		fmt.Fprintf(v.output, "%c", ch1)
		ch2 := v.RAM.Storage[i] >> 8
		if ch2 > 0 {
			fmt.Fprintf(v.output, "%c", ch2)
		}
	}
}

func (v *LC3CPU) trapHalt() {
	if _, err := fmt.Fprintln(v.output, "HALT"); err != nil {
		log.Fatalf("Can't write to device: %#v", v.output)
	}
	v.isRunning = false
}

func signExtend(x uint16, bitCount int) uint16 {
	if (x>>(bitCount-1))&1 == 1 {
		x |= 0xFFFF << bitCount
	}
	return x
}
