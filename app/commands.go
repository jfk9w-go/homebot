package app

import (
	"context"
	"strings"

	"github.com/jfk9w-go/homebot/core"
	telegram "github.com/jfk9w-go/telegram-bot-api"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type scopeCommands map[telegram.BotCommandScope]map[string]string

func (c scopeCommands) addGated(gate core.Gate, commands ...string) {
	if len(gate.UserIDs) > 0 {
		c.add(telegram.BotCommandScope{Type: telegram.BotCommandScopeAllPrivateChats}, commands...)
	}

	for chatID := range gate.ChatIDs {
		c.add(telegram.BotCommandScope{Type: telegram.BotCommandScopeChat, ChatID: chatID}, commands...)
	}
}

func (c scopeCommands) addDefault(commands ...string) {
	c.add(telegram.BotCommandScope{Type: telegram.BotCommandScopeDefault}, commands...)
}

func (c scopeCommands) add(scope telegram.BotCommandScope, commands ...string) {
	sc, ok := c[scope]
	if !ok {
		sc = make(map[string]string)
		c[scope] = sc
	}

	sc["start"] = "Get user & chat ID"
	for _, command := range commands {
		if strings.HasPrefix(command, "/") {
			command := command[1:]
			if _, ok := sc[command]; ok {
				logrus.Fatalf("duplicate command handler for %s", command)
			}

			sc[command] = humanizeKey(command)
		}
	}
}

func (c scopeCommands) set(ctx context.Context, client telegram.Client) error {
	for scope, commands := range c {
		scope := scope
		botCommands := make([]telegram.BotCommand, len(commands))
		i := 0
		for command, description := range commands {
			botCommands[i] = telegram.BotCommand{
				Command:     command,
				Description: description,
			}

			i++
		}

		if err := client.DeleteMyCommands(ctx, &scope); err != nil {
			return errors.Wrap(err, "delete commands")
		}

		if err := client.SetMyCommands(ctx, &scope, botCommands); err != nil {
			return errors.Wrap(err, "set commands")
		}
	}

	return nil
}
