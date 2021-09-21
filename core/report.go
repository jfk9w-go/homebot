package core

import "fmt"

type Report struct {
	lines []string
}

func NewReport() *Report {
	return &Report{lines: make([]string, 0)}
}

func (r *Report) Success(title, pattern string, args ...interface{}) {
	r.lines = append(r.lines, "✅ "+title+": "+fmt.Sprintf(pattern, args...))
}

func (r *Report) Error(title, pattern string, args ...interface{}) {
	errmsg := ""
	if len(args) > 0 {
		last := len(args) - 1
		if err, ok := args[last].(error); ok {
			errmsg = ": " + err.Error()
			args = args[:last]
		}
	}

	r.lines = append(r.lines, "❌ "+title+": "+fmt.Sprintf(pattern, args...)+errmsg)
}

func (r *Report) Dump() []string {
	return r.lines
}
