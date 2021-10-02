package tinkoff

import (
	"context"
	"strings"
	"time"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/core"
	"github.com/jfk9w-go/homebot/ext/tinkoff/external"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
)

type Context struct {
	Storage
	Reload time.Duration
}

type CommandListener struct {
	*Context
	flu.Clock
	*core.ControlButtons
	Credentials CredentialStore
	Executors   []Executor
}

func (l *CommandListener) AuthorizedUserIDs() map[telegram.ID]bool {
	userIDs := make(map[telegram.ID]bool, len(l.Credentials))
	for userID := range l.Credentials {
		userIDs[userID] = true
	}

	return userIDs
}

func (l *CommandListener) Update_bank_statement(ctx context.Context, tgclient telegram.Client, cmd *telegram.Command) error {
	cred, ok := l.Credentials[cmd.User.ID]
	if !ok {
		return errors.New("unknown user")
	}

	client, err := external.Authorize(ctx, cred.Username, cred.Password, func(ctx context.Context) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		m, err := tgclient.Ask(ctx, cmd.Chat.ID, &telegram.Text{Text: "Code:"}, nil)
		if err != nil {
			return "", err
		}

		return strings.Trim(m.Text, " \n"), nil
	})

	if err != nil {
		return err
	}

	report := core.NewJobReport()
	sync := &Sync{
		Context: l.Context,
		Client:  client,
		Now:     l.Now(),
		report:  report,
	}

	for _, executor := range l.Executors {
		if err := sync.Run(ctx, executor); err != nil {
			return err
		}
	}

	output := l.ControlButtons.Output(tgclient, cmd)
	for _, line := range report.Dump() {
		if err := output.WriteUnbreakable(ctx, line+"\n"); err != nil {
			return errors.Wrap(err, "send reply")
		}
	}

	if err := output.Flush(ctx); err != nil {
		return errors.Wrap(err, "send reply")
	}

	return nil
}
