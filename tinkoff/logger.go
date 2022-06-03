package tinkoff

import (
	"context"
	"fmt"
	"strings"

	"github.com/jfk9w-go/flu/logf"
	"github.com/jfk9w-go/flu/syncf"
	"github.com/jfk9w-go/telegram-bot-api/ext/html"
)

type logger interface {
	infof(ctx context.Context, msg string, args ...any)
	warnf(ctx context.Context, msg string, args ...any)
	errorf(ctx context.Context, msg string, args ...any)
	sub(name string) logger
}

var loggerLevelIcons = map[logf.Level]string{
	logf.Info:  "🔹",
	logf.Warn:  "🔸",
	logf.Error: "🔻",
}

type chapterLog struct {
	level logf.Level
	msg   string
	args  []any
}

type chapterLogs []chapterLog

func (ls *chapterLogs) add(log chapterLog) {
	*ls = append(*ls, log)
}

type telegramLogger struct {
	name     string
	level    logf.Level
	chapters map[string]*chapterLogs
	html     *html.Writer
	mu       syncf.Locker
}

func newTelegramLogger(html *html.Writer) *telegramLogger {
	return &telegramLogger{
		html:     html,
		level:    logf.Warn,
		chapters: make(map[string]*chapterLogs),
		mu:       syncf.Semaphore(nil, 1, 0),
	}
}

func (l *telegramLogger) String() string {
	return "tinkoff.logger"
}

func (l *telegramLogger) printf(ctx context.Context, level logf.Level, msg string, args ...any) {
	logf.Get(l).Logf(ctx, level, fmt.Sprintf("[%s] %s", l.name, msg), args...)

	if l.level.Skip(level) {
		return
	}

	_, cancel := l.mu.Lock(context.Background())
	defer cancel()

	logs, ok := l.chapters[l.name]
	if !ok {
		logs = new(chapterLogs)
		l.chapters[l.name] = logs
	}

	logs.add(chapterLog{
		level: level,
		msg:   msg,
		args:  args,
	})
}

func (l *telegramLogger) infof(ctx context.Context, msg string, args ...any) {
	l.printf(ctx, logf.Info, msg, args...)
}

func (l *telegramLogger) warnf(ctx context.Context, msg string, args ...any) {
	l.printf(ctx, logf.Warn, msg, args...)
}

func (l *telegramLogger) errorf(ctx context.Context, msg string, args ...any) {
	l.printf(ctx, logf.Error, msg, args...)
}

func (l *telegramLogger) sub(name string) logger {
	return &telegramLogger{
		name:     name,
		level:    l.level,
		chapters: l.chapters,
		html:     l.html,
		mu:       l.mu,
	}
}

func (l *telegramLogger) flush() {
	if l.name != "" {
		return
	}

	_, cancel := l.mu.Lock(context.Background())
	defer cancel()

	for chapter, logs := range l.chapters {
		l.html.Bold("\n▫️ %s", chapter)
		for _, log := range *logs {
			l.html.Text("\n"+loggerLevelIcons[log.level]+"️️ "+strings.Trim(log.msg, "\n"), log.args...)
		}
	}

	if len(l.chapters) == 0 {
		l.html.Text("✔️")
	}
}
