package core

import "fmt"

type JobReport struct {
	lines []string
}

func NewJobReport() *JobReport {
	return &JobReport{lines: make([]string, 0)}
}

func (r *JobReport) Success(title, pattern string, args ...interface{}) {
	r.lines = append(r.lines, "✔️ "+title+": "+fmt.Sprintf(pattern, args...))
}

func (r *JobReport) Error(title, pattern string, args ...interface{}) {
	errmsg := ""
	if len(args) > 0 {
		last := len(args) - 1
		if err, ok := args[last].(error); ok {
			errmsg = ": " + err.Error()
			args = args[:last]
		}
	}

	r.lines = append(r.lines, "✖️ "+title+": "+fmt.Sprintf(pattern, args...)+errmsg)
}

func (r *JobReport) Dump() []string {
	return r.lines
}
