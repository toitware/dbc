// Copyright (C) 2021 Toitware ApS. All rights reserved.

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	// TODO: Once https://github.com/einride/can-go/pull/42 is in, use 'einride/can-go' again and remove 'toitware/can-go'
	"github.com/toitware/can-go/pkg/dbc"

	"github.com/spf13/cobra"
	"github.com/toitware/dbc/dbc-gen/toit"
)

func main() {
	cmd := &cobra.Command{
		Use:   "dbc-gen",
		Short: "generate toit stubs from DBC files",
		RunE:  genStubs,
	}
	cmd.Flags().StringP("output", "o", "-", "output file for the generate code. Default to '-', stdout.")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func genStubs(cmd *cobra.Command, args []string) error {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}

	var msgs []*dbc.MessageDef
	multiplexerValues := map[dbc.MessageID][]*dbc.SignalMultiplexValueDef{}
	valueDescriptions := []*dbc.ValueDescriptionsDef{}

	for _, file := range args {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}

		p := dbc.NewParser(file, data)

		if err := p.Parse(); err != nil {
			return err
		}

		for _, d := range p.File().Defs {
			switch def := d.(type) {
			case *dbc.MessageDef:
				msgs = append(msgs, def)
			case *dbc.SignalMultiplexValueDef:
				list := multiplexerValues[def.MessageID]
				list = append(list, def)
				multiplexerValues[def.MessageID] = list
			case *dbc.ValueDescriptionsDef:
				valueDescriptions = append(valueDescriptions, def)
			}
		}
	}

	var buffer bytes.Buffer
	w := toit.NewWriter(&buffer)
	w.Import("dbc")

	for _, msg := range msgs {
		if err := processMessage(msg, multiplexerValues[msg.MessageID], w); err != nil {
			return err
		}
	}

	for _, valueDescription := range valueDescriptions {
		if err := processValueDescription(w, valueDescription); err != nil {
			return err
		}
	}

	if output == "-" {
		_, err := io.Copy(os.Stdout, &buffer)
		return err
	} else {
		return ioutil.WriteFile(output, buffer.Bytes(), 0644)
	}
}

type Multiplex struct {
	MultiplexerSwitch dbc.Identifier
	Value             uint64
}
type Message struct {
	Message              *dbc.MessageDef
	ExtendedMultiplexers []*dbc.SignalMultiplexValueDef
	SignalsByName        map[dbc.Identifier]dbc.SignalDef
	MultiplexedBy        map[dbc.Identifier][]dbc.Identifier
	MultiplexSignals     map[dbc.Identifier]*Multiplex
}

func processMessage(msg *dbc.MessageDef, extMultiplexers []*dbc.SignalMultiplexValueDef, w *toit.Writer) error {
	message := &Message{
		Message:              msg,
		ExtendedMultiplexers: extMultiplexers,
		SignalsByName:        map[dbc.Identifier]dbc.SignalDef{},
		MultiplexedBy:        map[dbc.Identifier][]dbc.Identifier{},
		MultiplexSignals:     map[dbc.Identifier]*Multiplex{},
	}
	for _, s := range msg.Signals {
		message.SignalsByName[s.Name] = s
	}

	for _, m := range extMultiplexers {
		message.MultiplexSignals[m.Signal] = &Multiplex{
			MultiplexerSwitch: m.MultiplexerSwitch,
			Value:             m.RangeStart,
		}
		message.MultiplexedBy[m.MultiplexerSwitch] = append(message.MultiplexedBy[m.MultiplexerSwitch], m.Signal)
	}
	var multiplexerSwitch dbc.SignalDef
	for _, s := range msg.Signals {
		if s.IsMultiplexerSwitch && !s.IsMultiplexed {
			multiplexerSwitch = s
			break
		}
	}
	var baseSignals []dbc.Identifier
	for _, s := range msg.Signals {
		if s.IsMultiplexed {
			if _, ok := message.MultiplexSignals[s.Name]; !ok {
				message.MultiplexSignals[s.Name] = &Multiplex{
					MultiplexerSwitch: multiplexerSwitch.Name,
					Value:             s.MultiplexerSwitch,
				}
				message.MultiplexedBy[multiplexerSwitch.Name] = append(message.MultiplexedBy[multiplexerSwitch.Name], s.Name)
			}
		} else {
			baseSignals = append(baseSignals, s.Name)
		}
	}

	processMultiplexMessage(message, multiplexerSwitch, baseSignals, nil, "", w)

	w.StartClass(string(msg.Name)+"Decoder", "", "dbc.Decoder")

	w.StartFunctionDecl("id")
	w.EndFunctionDecl("int")
	w.ReturnStart()
	w.Argument(string(msg.Name) + ".ID")
	w.ReturnEnd()
	w.EndFunction()

	w.StartFunctionDecl("decode")
	w.Parameter("reader", "dbc.Reader")
	w.EndFunctionDecl(string(msg.Name))

	processDecodeMessage(message, multiplexerSwitch, baseSignals, nil, "", w)

	w.EndFunction()

	w.EndClass()

	return nil
}

func processMultiplexMessage(msg *Message, signal dbc.SignalDef, signals []dbc.Identifier, baseSignals []dbc.Identifier, baseName string, w *toit.Writer) {
	newName := string(msg.Message.Name)
	if baseName != "" {
		newName = baseName + "_" + string(signal.Name)
	}

	w.StartClass(newName, baseName)

	w.StaticConst("ID", "int", strconv.FormatUint(uint64(msg.Message.MessageID.ToCAN()), 10))
	w.NewLine()

	for _, s := range signals {
		w.Variable(signalName(s), "num", "0")
		w.EndAssignment()
	}

	w.NewLine()

	w.StartConstructorDecl("")
	w.EndConstructorDecl()
	w.EndConstructor()

	if len(baseSignals)+len(signals) > 0 {
		w.StartConstructorDecl("")

		for _, s := range baseSignals {
			w.Parameter(signalName(s), "")
		}

		for _, s := range signals {
			w.Parameter("."+signalName(s), "")
		}
		w.EndConstructorDecl()
		w.StartCall("super")
		for _, s := range baseSignals {
			w.Argument(signalName(s))
		}
		w.EndCall()
		w.EndConstructor()
	}

	w.EndClass()
	w.NewLine()

	if signal.IsMultiplexerSwitch {
		for _, s := range msg.MultiplexedBy[signal.Name] {
			processMultiplexMessage(msg, msg.SignalsByName[s], []dbc.Identifier{s}, append(baseSignals, signals...), newName, w)
		}
	}
}

func processDecodeMessage(msg *Message, signal dbc.SignalDef, signals []dbc.Identifier, baseSignals []dbc.Identifier, baseName string, w *toit.Writer) {
	name := string(msg.Message.Name)
	if baseName != "" {
		name = baseName + "_" + string(signal.Name)
	}

	args := ""
	for _, s := range baseSignals {
		args += " " + string(s)
	}
	for _, s := range signals {
		args += " " + string(s)
	}

	for _, s := range signals {
		processSignal(w, msg.Message, msg.SignalsByName[s])
	}

	if signal.IsMultiplexerSwitch {
		for _, s := range msg.MultiplexedBy[signal.Name] {
			m := msg.MultiplexSignals[s]
			w.Literal(fmt.Sprintf("if %s == %d", signal.Name, m.Value))
			w.StartBlock(false)

			processDecodeMessage(msg, msg.SignalsByName[s], []dbc.Identifier{s}, append(baseSignals, signals...), name, w)

			w.EndBlock(false)
		}

		w.Variable("message", name, name+args)
		w.ReturnStart()
		w.Argument("message")
		w.ReturnEnd()
	} else {
		w.Variable("message", name, name+args)
		w.ReturnStart()
		w.Argument("message")
		w.ReturnEnd()
	}
}

func startBit(s dbc.SignalDef) int {
	if !s.IsBigEndian {
		return int(s.StartBit)
	}

	startBit := 8 * (s.StartBit / 8)
	startBit += s.StartBit%8 + 1
	startBit -= s.Size
	return int(startBit)
}

func processSignal(w *toit.Writer, msg *dbc.MessageDef, s dbc.SignalDef) {
	w.Variable(signalName(s.Name), "num", "0")
	w.StartAssignment(signalName(s.Name))
	w.StartCall(" reader.read")
	w.Argument(strconv.Itoa(startBit(s)))
	w.Argument(strconv.Itoa(int(s.Size)))
	if s.IsSigned {
		w.Argument("--signed")
	}
	w.EndAssignment()
	w.EndCall()
	// Check if it requires to be converted.
	if s.Maximum != float64(int(1)<<(s.Size-1)) || s.Minimum != 0 || s.Factor != 1 || s.Pos.Offset != 0 {
		w.StartAssignment(signalName(s.Name))
		w.StartCall(" dbc.to_physical")
		w.Argument(signalName(s.Name))
		w.Argument(strconv.FormatFloat(s.Factor, 'g', -1, 64))
		w.Argument(strconv.FormatFloat(s.Offset, 'g', -1, 64))
		w.Argument(strconv.FormatFloat(s.Minimum, 'g', -1, 64))
		w.Argument(strconv.FormatFloat(s.Maximum, 'g', -1, 64))
		w.EndCall()
		w.EndAssignment()
		w.NewLine()
	}
}

func processValueDescription(w *toit.Writer, valueDescription *dbc.ValueDescriptionsDef) error {
	w.NewLine()
	prefix := toit.ToSnakeCase(signalName(valueDescription.SignalName))
	for _, value := range valueDescription.ValueDescriptions {
		name := strings.ToUpper(prefix + "_" + toit.ToSnakeCase(value.Description))
		w.Const(name, "num", strconv.FormatFloat(value.Value, 'g', -1, 64))
	}
	return nil
}

func signalName(name dbc.Identifier) string {
	return string(name)
}
