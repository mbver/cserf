package utils

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type Output struct {
	out          io.Writer
	err          io.Writer
	infoPrefix   string
	errorPrefix  string
	resultPrefix string
}

func NewOutput(out, err io.Writer, infoPrefix, errPrefix, resultPrefix string) *Output {
	return &Output{
		out:          out,
		err:          err,
		infoPrefix:   infoPrefix,
		errorPrefix:  errPrefix,
		resultPrefix: resultPrefix,
	}
}

func DefaultOutput(out, err io.Writer) *Output {
	return NewOutput(
		out,
		err,
		">> ",
		"***** error: ",
		"====> output: ",
	)
}

func CreateOutputFromCmd(cmd *cobra.Command) *Output {
	return DefaultOutput(cmd.OutOrStdout(), cmd.OutOrStderr())
}

func (o *Output) Result(name string, obj interface{}) {
	if obj == nil {
		fmt.Fprintf(o.out, "%s%s\n", o.resultPrefix, name)
		return
	}
	jbytes, err := json.MarshalIndent(obj, "", " ")
	if err != nil {
		o.Error(err)
		return
	}
	fmt.Fprintf(o.out, "%s%s: %s\n", o.resultPrefix, name, string(jbytes))
}
func (o *Output) Info(s string) {
	fmt.Fprintf(o.out, "%s%s\n", o.infoPrefix, s)
}

func (o *Output) Infof(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.out, "%s%s\n", o.infoPrefix, s)
}
func (o *Output) Error(err error) {
	fmt.Fprintf(o.err, "%s%s\n", o.errorPrefix, err)
}
