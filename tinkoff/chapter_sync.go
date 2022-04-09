package tinkoff

import (
	"context"
	"homebot/tinkoff/external"
	"time"

	"github.com/jfk9w-go/flu/logf"
	"github.com/jfk9w-go/flu/syncf"
	"github.com/jfk9w-go/telegram-bot-api/ext/html"
)

type chapterSync struct {
	client  *external.Client
	chapter Chapter
	period  time.Duration
	log     func() logf.Interface
	quiet   bool
}

func (s chapterSync) run(ctx context.Context, html *html.Writer) error {
	s.log().Tracef(ctx, "syncing chapter %s", s.chapter)
	subs, count, err := s.chapter.Sync(ctx, s.client, s.period)
	s.log().Resultf(ctx, logf.Debug, logf.Warn, "chapter %s sync (%d): %v", s.chapter, count, err)

	if err := s.write(html, count, err); err != nil {
		return err
	}

	for _, sub := range subs {
		sync := chapterSync{
			client:  s.client,
			chapter: sub,
			period:  s.period,
			log:     s.log,
			quiet:   false,
		}

		if err := sync.run(ctx, html); err != nil {
			return err
		}
	}

	return nil
}

func (s chapterSync) write(html *html.Writer, count int, err error) error {
	if s.quiet && err == nil {
		return nil
	}

	html.Bold(s.chapter.Title())
	if err != nil {
		html.Text(" → %v\n", err)
	} else {
		html.Text(" → %d items synced\n", count)
	}

	if syncf.IsContextRelated(err) {
		return err
	}

	return nil
}
