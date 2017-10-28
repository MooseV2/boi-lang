package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
)

// These type definitions make it possible to
// end every type with "boi"
type IntyBoi int
type FloatyBoi float64
type StringyBoi string

func boiError(boiInputs ...interface{}) {
	fmt.Print("\033[31;1mBoi! ")
	fmt.Print(boiInputs...)
	fmt.Println(", boi\033[0m")
}

func main() {
	boiArgs := os.Args[1:] // boi
	if len(boiArgs) < 1 {
		boiError("Usage: boi script.boi\n")
		os.Exit(1)
	}
	err := boiBoi(boiArgs[0]) // boi
	if err != nil {
		boiError(err)
	}
}

func boiBoi(boiFilename string) error {
	if boiFilename[len(boiFilename)-3:] != "boi" {
		return fmt.Errorf(
			"boi %s: MUST end with 'boi'", boiFilename,
		)
	}
	boiFile, err := os.Open(boiFilename)
	if err != nil {
		return err
	}

	code, err := ioutil.ReadAll(boiFile)
	if err != nil {
		return err
	}

	lex := NewBoiInterpreter(code)
	if err := lex.Run(); err != nil {
		return err
	}

	return nil
}

type BoiVar struct {
	data []byte
}

const (
	BoiTokenValue = 1 // A string
	BoiTokenVar   = 2
	BoiTokenBoi   = 3 // End of statement
)

const (
	// BoiStateStatement means we're expecting a statement
	BoiStateStatement IntyBoi = 0 // boi
)

// Enumerated list of "source types"
const (
	BoiSourceLocal  = 1
	BoiSourceReturn = 2
)

type Token struct {
	BoiType IntyBoi

	// For strings or variable names
	BoiValue []byte

	// Source context (for variables)
	BoiSource int
}

type BoiFunc interface {
	Do(args []BoiVar) error
}

type BoiContext struct {
	functions map[string]BoiFunc
	variables map[string]BoiVar
	parentCtx *BoiContext
	returnCtx *BoiContext
}

func (ctx *BoiContext) Call(fname string, args []BoiVar) error {
	f, exists := ctx.functions[fname]
	if !exists {
		if ctx.parentCtx == nil {
			return fmt.Errorf("call to undefined function %s", fname)
		} else {
			return ctx.parentCtx.Call(fname, args)
		}
	}
	return f.Do(args)
}

type BoiInterpreter struct {
	input []byte
	pos   IntyBoi
	state IntyBoi

	rIsBoiVar *regexp.Regexp
	rIsRetVar *regexp.Regexp
	rIsBoi    *regexp.Regexp

	context *BoiContext
}

func NewBoiInterpreter(input []byte) *BoiInterpreter {
	rootContext := &BoiContext{
		map[string]BoiFunc{},
		map[string]BoiVar{},
		nil, nil,
	}

	boi := &BoiInterpreter{
		input, 0, BoiStateStatement,
		nil, nil, nil,
		rootContext,
	}
	boi.rIsBoiVar = regexp.MustCompile("^boi:[A-z][A-z0-9]*")
	boi.rIsRetVar = regexp.MustCompile("^ret:[A-z][A-z0-9]*")
	boi.rIsBoi = regexp.MustCompile("^boi[\\s\\n]")

	// Add internal functions
	boi.context.functions["say"] = BoiFuncSay{}
	boi.context.functions["set"] = BoiFuncSet{boi}
	boi.context.functions["cat"] = BoiFuncCat{boi}

	return boi
}

func (boi *BoiInterpreter) subContext() *BoiContext {
	ctx := &BoiContext{
		map[string]BoiFunc{},
		map[string]BoiVar{},
		boi.context, nil,
	}
	boi.context = ctx
	return ctx
}

func (boi *BoiInterpreter) returnContext() error {
	returnCtx := boi.context
	boi.context = boi.context.parentCtx
	if boi.context == nil {
		return errors.New("returned to nil context")
	}
	boi.context.returnCtx = returnCtx
	return nil
}

func (boi *BoiInterpreter) Run() error {
	for {
		if boi.whitespace() {
			return nil
		}
		if err := boi.doStatement(); err != nil {
			return err
		}
	}
	return nil
}

func (boi *BoiInterpreter) whitespace() bool {
	if !(boi.pos < IntyBoi(len(boi.input)-1)) {
		return true // reached EOF
	}
	for ; boi.pos < IntyBoi(len(boi.input)); boi.pos++ {
		//
		if !(boi.input[boi.pos] == ' ' ||
			boi.input[boi.pos] == '\n' ||
			boi.input[boi.pos] == '\t') {
			return false
		}
	}
	return true
}

func (boi *BoiInterpreter) noeof(hasEof bool) error {
	if hasEof {
		return errors.New("unexpected EOF")
	}
	return nil
}

func (boi *BoiInterpreter) doStatement() error {
	stmt, err := boi.getStatement()
	if err != nil {
		return err
	}
	return boi.ExecStmt(stmt)
}

func (boi *BoiInterpreter) getStatement() (*BoiStatement, error) {
	op := string(boi.input[boi.pos : boi.pos+4])
	switch op {
	case "boi!":
		boi.pos += 4
		boi.noeof(boi.whitespace())
		tokens := []Token{}
		tokBytes := []BoiVar{}
		for {
			boi.noeof(boi.whitespace())
			if token, err := boi.eatToken(); err == nil {
				if token.BoiType != BoiTokenBoi {
					tokens = append(tokens, token)
					tokBytes = append(tokBytes, BoiVar{token.BoiValue})
				} else {
					break
				}
			} else {
				return nil, err
			}
		}
		return &BoiStatement{
			BoiOpCall, tokens,
		}, nil
	default:
		return nil, errors.New("unexpected")
	}
	return nil, errors.New("unexpected")
}

func (boi *BoiInterpreter) Call(identifier string, args []BoiVar) error {
	if f, exists := boi.context.functions[identifier]; exists {
		err := f.Do(args)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("function %s: not found", identifier)
	}
	return nil
}

func (boi *BoiInterpreter) getValueOf(tok Token) (BoiVar, bool) {
	switch tok.BoiType {
	case BoiTokenValue:
		return BoiVar{tok.BoiValue}, true
	case BoiTokenVar:
		context := boi.context
		if tok.BoiSource == BoiSourceReturn {
			context = boi.context.returnCtx
		}

		if context == nil {
			// TODO: Raise error if boi.context is strict context
			return BoiVar{}, false
		}

		identifier := string(tok.BoiValue)
		value, exists := context.variables[identifier]
		if !exists {
			// TODO: Raise error if strict context
		}
		return value, exists
	}
	return BoiVar{}, false
}

func (boi *BoiInterpreter) eatToken() (Token, error) {
	if !(boi.pos < IntyBoi(len(boi.input))) {
		return Token{}, errors.New("unexpected EOF")
	}

	isBoi := boi.rIsBoi.Match(boi.input[boi.pos:])
	if isBoi {
		boi.pos += 4
		t := Token{
			BoiType:  BoiTokenBoi,
			BoiValue: []byte{},
		}
		return t, nil
	}

	token := Token{
		BoiType:   BoiTokenValue,
		BoiValue:  []byte{},
		BoiSource: BoiSourceLocal,
	}

	isBoiVar := boi.rIsBoiVar.Match(boi.input[boi.pos:])
	isRetVar := boi.rIsRetVar.Match(boi.input[boi.pos:])
	if isBoiVar || isRetVar {
		boi.pos += 4
		token.BoiType = BoiTokenVar
	}
	if isRetVar {
		token.BoiSource = BoiSourceReturn
	}

	if boi.input[boi.pos] == '"' {
		boi.pos++ // otherwise we'll stop at the first quote
		value := []byte{}
		literal := false
		for ; boi.pos < IntyBoi(len(boi.input)); boi.pos++ {
			c := boi.input[boi.pos]
			if literal {
				value = append(value, c)
			} else {
				if c == '\\' {
					literal = true
				} else if c == '"' {
					boi.pos++ // don't forget to go past this quote
					break
				} else {
					value = append(value, c)
				}
			}
		}
		token.BoiValue = value
		return token, nil
	}
	if true {
		value := []byte{}
		literal := false
		for ; boi.pos < IntyBoi(len(boi.input)); boi.pos++ {
			c := boi.input[boi.pos]
			if literal {
				value = append(value, c)
			} else {
				if c == '\\' {
					literal = true
				} else if c == ' ' {
					boi.pos++ // don't forget to go past this space
					break
				} else {
					value = append(value, c)
				}
			}
		}
		token.BoiValue = value
		return token, nil
	}
	return Token{}, errors.New("unexpected token")
}
