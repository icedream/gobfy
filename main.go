package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	InstMoveRight byte = '>'
	InstMoveLeft       = '<'
	InstIncrement      = '+'
	InstDecrement      = '-'
	InstOutput         = '.'
	InstInput          = ','
	InstLoopStart      = '['
	InstLoopEnd        = ']'
)

var (
	app = kingpin.New("gobfy", "Yet another interpreter for Brainfuck programs.")

	argInput = app.Arg("input", "The source file of the program to execute.").Required().ExistingFile()

	flagDebug = app.Flag("debug", "Indicates whether to display information about the current state before executing each instruction.").Bool()
)

const (
	DefaultPageSize = 1024
)

type Closure struct {
	Skip  bool
	Root  bool
	Start int
}

type Processor struct {
	Data        []byte
	DataPointer int

	Debug bool

	stdin *bufio.Reader

	instructionPointer int
	instructionBuffer  []byte

	closures []*Closure
}

func NewProcessor() *Processor {
	return &Processor{
		Data:  make([]byte, DefaultPageSize),
		stdin: bufio.NewReader(os.Stdin),
		closures: []*Closure{
			&Closure{Root: true},
		},
		instructionBuffer: []byte{},
	}
}

func (p *Processor) Stdin(r io.Reader) {
	p.stdin = bufio.NewReader(r)
}

func (p *Processor) ensureDataSize() {
	if p.DataPointer >= len(p.Data) {
		// Increase data array, lock to next page size
		nextPagedSize := (1 + (p.DataPointer / DefaultPageSize)) * DefaultPageSize
		p.Data = append(p.Data, make([]byte, 1+nextPagedSize-len(p.Data))...)
	}
}

func (p *Processor) Load(instructions []byte) {
	p.instructionBuffer = instructions
	p.instructionPointer = 0
}

func (p *Processor) Execute() {
	for p.instructionPointer < len(p.instructionBuffer) {
		instruction := p.instructionBuffer[p.instructionPointer]

		if p.Debug {
			log.Printf("exec 0x%[2]x = %[1]q, data: 0x%[3]x = %[3]q (0x%[4]x), reserved data size: %[5]d B",
				instruction,
				p.instructionPointer,
				p.Data[p.DataPointer],
				p.DataPointer,
				len(p.Data))
		}

		switch instruction {
		case InstMoveRight:
			p.MoveRight()
		case InstMoveLeft:
			p.MoveLeft()
		case InstDecrement:
			p.Decrement()
		case InstIncrement:
			p.Increment()
		case InstInput:
			p.Input()
		case InstOutput:
			p.Output()
		case InstLoopStart:
			p.StartLoop()
		case InstLoopEnd:
			p.EndLoop()
		default:
			// Skip
		}

		p.instructionPointer++
	}
}

func (p *Processor) Current() byte {
	return p.Data[p.DataPointer]
}

func (p *Processor) Increment() {
	if p.closures[0].Skip {
		return
	}

	p.Data[p.DataPointer]++
}

func (p *Processor) Decrement() {
	if p.closures[0].Skip {
		return
	}

	p.Data[p.DataPointer]--
}

func (p *Processor) MoveRight() {
	if p.closures[0].Skip {
		return
	}

	p.DataPointer++
	p.ensureDataSize()
}

func (p *Processor) MoveLeft() {
	if p.closures[0].Skip {
		return
	}

	if p.DataPointer == 0 {
		log.Fatal("can not move data pointer left, already at beginning of data")
	}

	p.DataPointer--
}

func (p *Processor) Output() {
	if p.closures[0].Skip {
		return
	}

	fmt.Printf("%c", rune(p.Data[p.DataPointer]))
}

func (p *Processor) Input() {
	if p.closures[0].Skip {
		return
	}

	input, err := p.stdin.ReadByte()
	if err != nil {
		log.Fatal(err)
	}
	p.Data[p.DataPointer] = input
}

func (p *Processor) StartLoop() {
	p.closures = append([]*Closure{
		&Closure{
			Start: p.instructionPointer,
			Skip:  p.Data[p.DataPointer] == 0,
		},
	}, p.closures...)
}

func (p *Processor) EndLoop() {
	if len(p.closures) <= 1 {
		log.Fatal("unexpected end of closure, not in any closure")
	}

	currentClosure := p.closures[0]

	if !currentClosure.Skip {
		if p.Data[p.DataPointer] > 0 {
			p.instructionPointer = currentClosure.Start
			return
		}
	}

	p.closures = p.closures[1:]
}

func (p *Processor) ExpectEnd() {
	if len(p.closures) > 1 {
		log.Fatal("unexpected end of instructions, still in a closure")
	}
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	inputFilePath := *argInput

	// Open BF source code
	input, err := ioutil.ReadFile(inputFilePath)
	if err != nil {
		log.Fatal(err)
	}

	p := NewProcessor()

	if flagDebug != nil {
		p.Debug = *flagDebug
	}

	p.Load(input)
	p.Execute()
	p.ExpectEnd()
}
