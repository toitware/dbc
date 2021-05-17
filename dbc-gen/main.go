package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"strconv"

	"go.einride.tech/can/pkg/dbc"

	"github.com/toitware/dbc.git/dbc-gen/toit"
)

func main() {
	file := "robin.dbc"

	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	p := dbc.NewParser(file, data)

	if err := p.Parse(); err != nil {
		log.Fatal(err)
	}

	var msgs []*dbc.MessageDef
	for _, d := range p.File().Defs {
		switch def := d.(type) {
		case *dbc.MessageDef:
			msgs = append(msgs, def)
		}
	}

	var output bytes.Buffer
	w := toit.NewWriter(&output)
	w.ImportAs(".dbc", "dbc")

	for _, msg := range msgs {
		if err := processMessage(msg, w); err != nil {
			log.Fatal(err)
		}
	}

	if err := ioutil.WriteFile("out.toit", output.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}

func processMessage(msg *dbc.MessageDef, w *toit.Writer) error {
	w.StartClass(string(msg.Name), "dbc.Message")

	w.StaticConst("ID", "int", strconv.Itoa(int(msg.MessageID.ToCAN())))
	w.NewLine()

	for _, s := range msg.Signals {
		w.Variable(signalName(s.Name), "num", "0")
		w.EndAssignment()
	}

	w.EndClass()
	w.NewLine()

	w.StartClass(string(msg.Name)+"Decoder", "", "dbc.Decoder")

	w.StartFunctionDecl("id")
	w.EndFunctionDecl("int")
	w.ReturnStart()
	w.Argument(string(msg.Name) + ".ID")
	w.ReturnEnd()
	w.EndFunction()

	w.StartFunctionDecl("decode")
	w.Parameter("reader", "dbc.BitReader")
	w.EndFunctionDecl(string(msg.Name))

	w.Variable("message", string(msg.Name), string(msg.Name))
	w.Variable("raw", "", "0")

	for _, s := range msg.Signals {
		w.StartAssignment("raw")
		w.StartCall("reader.read")
		w.Argument(strconv.Itoa(int(s.StartBit)))
		w.Argument(strconv.Itoa(int(s.Size)))
		if s.IsSigned {
			w.Argument("--signed")
		}
		w.EndAssignment()
		w.EndCall()

		w.StartAssignment("message." + signalName(s.Name))
		w.StartCall("dbc.to_physical")
		w.Argument("raw")
		w.Argument(strconv.FormatFloat(s.Factor, 'g', -1, 64))
		w.Argument(strconv.FormatFloat(s.Offset, 'g', -1, 64))
		w.Argument(strconv.FormatFloat(s.Minimum, 'g', -1, 64))
		w.Argument(strconv.FormatFloat(s.Maximum, 'g', -1, 64))
		w.EndCall()
		w.EndAssignment()
	}

	w.ReturnStart()
	w.Argument("message")
	w.ReturnEnd()
	w.EndFunction()

	w.EndClass()

	return nil
}

func signalName(name dbc.Identifier) string {
	return "signal_" + string(name)
}
