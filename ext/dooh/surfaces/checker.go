package surfaces

import (
	"context"
	"fmt"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/ext/dooh"
	"github.com/jfk9w-go/telegram-bot-api"
	"github.com/jfk9w-go/telegram-bot-api/ext/html"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type SurfaceDictionary struct {
	Surfaces map[string]Surface
}

type Checker struct {
	*dooh.Service
	ApiClient *ApiClient
	File      flu.File

	mu     flu.Mutex
	work   flu.WaitGroup
	cancel func()
}

func (c *Checker) RunInBackground(ctx context.Context, every time.Duration) error {
	if c.cancel != nil {
		return nil
	}

	if err := c.run(ctx); err != nil {
		return err
	}

	c.cancel = c.work.Go(ctx, func(ctx context.Context) {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.run(ctx); err != nil {
					return
				}
			}
		}
	})

	return nil
}

func (c *Checker) Close() error {
	if c.cancel != nil {
		c.cancel()
		c.work.Wait()
	}

	return nil
}

func (c *Checker) Update_surfaces(ctx context.Context, tgclient telegram.Client, cmd *telegram.Command) error {
	if err := c.run(ctx); err != nil {
		return errors.Wrap(err, "update surfaces")
	}

	return cmd.Reply(ctx, tgclient, "OK")
}

func (c *Checker) run(ctx context.Context) error {
	writer := &html.Writer{
		Context: ctx,
		Out:     c.NewOutput(),
	}

	if err := c.runWith(ctx, writer); err != nil {
		if flu.IsContextRelated(err) {
			return err
		}

		writer.Text(err.Error())
	}

	if err := writer.Flush(); err != nil {
		if flu.IsContextRelated(err) {
			return err
		}

		logrus.WithField("service", "dooh").Errorf("flush html writer: %s", err)
	}

	return nil
}

func (c *Checker) runWith(ctx context.Context, html *html.Writer) error {
	defer c.mu.Lock().Unlock()
	surfaces, err := c.ApiClient.Surfaces(ctx)
	if err != nil {
		return errors.Wrap(err, "get surfaces")
	}

	existing := new(SurfaceDictionary)
	if ok, err := c.File.Exists(); ok {
		if err := flu.DecodeFrom(c.File, flu.Gob(existing)); err != nil {
			return errors.Wrap(err, "decode file")
		}
	} else if err != nil {
		return errors.Wrap(err, "exists")
	}

	updated := &SurfaceDictionary{
		Surfaces: make(map[string]Surface),
	}

	for _, surface := range surfaces {
		updated.Surfaces[surface.ID] = surface
		externalID := fmt.Sprintf("[%s %s]",
			surface.Attributes.Network,
			surface.Attributes.SurfaceID)
		url := ResourceURL("surfaces", surface.ID)
		if entry, ok := existing.Surfaces[surface.ID]; ok {
			if entry.Attributes.UpdatedAt != surface.Attributes.UpdatedAt {
				if entry.Attributes.DeletedAt == nil && surface.Attributes.DeletedAt != nil {
					html.Bold("DELETED")
				} else if entry.Attributes.DeletedAt != nil && surface.Attributes.DeletedAt == nil {
					html.Bold("RESTORED")
				} else {
					html.Bold("UPDATED")
				}
			} else {
				continue
			}
		} else {
			html.Bold("ADDED")
		}

		html.Text(" ").Link(externalID, url).Text(" " + surface.Attributes.Name).Text("\n")
	}

	if err := html.Flush(); err != nil {
		return errors.Wrap(err, "flush html writer")
	}

	if err := flu.EncodeTo(flu.Gob(updated), c.File); err != nil {
		return errors.Wrap(err, "save surfaces")
	}

	logrus.WithField("service", "dooh").Info("updated surfaces")
	return nil
}
