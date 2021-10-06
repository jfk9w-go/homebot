package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/jfk9w-go/telegram-bot-api/ext/output"
)

type JobReport struct {
	lines []string
}

func NewJobReport() *JobReport {
	return &JobReport{lines: make([]string, 0)}
}

func (r *JobReport) write(symbol, title, pattern string, args ...interface{}) {
	var b strings.Builder
	if symbol != "" {
		b.WriteString(symbol + " ")
	}

	b.WriteString(title + "\n")
	if pattern != "" {
		errmsg := ""
		if len(args) > 0 {
			last := len(args) - 1
			if err, ok := args[last].(error); ok {
				errmsg = "\n" + err.Error()
				args = args[:last]
			}

			pattern = fmt.Sprintf(pattern, args...)
		}

		b.WriteString(pattern + errmsg + "\n")
	}

	r.lines = append(r.lines, b.String())
}

func (r *JobReport) Title(title string) {
	r.write("", title, "")
}

func (r *JobReport) Info(title, pattern string, args ...interface{}) {
	r.write("ℹ️", title, pattern, args...)
}

func (r *JobReport) Warn(title, pattern string, args ...interface{}) {
	r.write("⚠️", title, pattern, args...)
}

func (r *JobReport) Error(title, pattern string, args ...interface{}) {
	r.write("🛑", title, pattern, args...)
}

func (r *JobReport) DumpTo(ctx context.Context, output *output.Paged) error {
	for _, line := range r.lines {
		if err := output.WriteUnbreakable(ctx, line+"\n"); err != nil {
			return err
		}
	}

	return output.Flush(ctx)
}
