package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	"go.einride.tech/can/pkg/dbc"

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
			}
		}
	}

	var buffer bytes.Buffer
	w := toit.NewWriter(&buffer)
	w.Import("dbc")

	for _, msg := range msgs {
		if err := processMessage(msg, w); err != nil {
			return err
		}
	}

	if output == "-" {
		_, err := io.Copy(os.Stdout, &buffer)
		return err
	} else {
		return ioutil.WriteFile("out.toit", buffer.Bytes(), 0644)
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
	w.Parameter("reader", "dbc.Reader")
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
