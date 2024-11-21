package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Output struct {
	output       io.Writer
	infoPrefix   string
	errorPrefix  string
	resultPrefix string
}

func NewOutput(w io.Writer, infoPrefix, errPrefix, resultPrefix string) *Output {
	return &Output{
		output:       w,
		infoPrefix:   infoPrefix,
		errorPrefix:  errPrefix,
		resultPrefix: resultPrefix,
	}
}

func DefaultOutput() *Output {
	return NewOutput(
		os.Stdout,
		">> ",
		"***** error: ",
		"====> output: ",
	)
}

func (o *Output) Result(name string, obj interface{}) {
	if obj == nil {
		fmt.Fprintf(o.output, "%s%s\n", o.resultPrefix, name)
		return
	}
	jbytes, err := json.MarshalIndent(obj, "", " ")
	if err != nil {
		o.Error(err)
		return
	}
	fmt.Fprintf(o.output, "%s%s: %s\n", o.resultPrefix, name, string(jbytes))
}
func (o *Output) Info(s string) {
	fmt.Fprintf(o.output, "%s%s\n", o.infoPrefix, s)
}

func (o *Output) Infof(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.output, "%s%s\n", o.infoPrefix, s)
}
func (o *Output) Error(err error) {
	fmt.Fprintf(o.output, "%s%s\n", o.errorPrefix, err)
}
