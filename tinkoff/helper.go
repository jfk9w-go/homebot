package tinkoff

import (
	"fmt"

	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/telegram-bot-api/ext/app"
	"github.com/pkg/errors"
)

func Run(app app.Interface) (bool, error) {
	globalConfig := new(struct {
		Tinkoff struct {
			Show, Generate bool
			Data           flu.File
			Credentials    CredentialStore
		}
	})

	if err := app.GetConfig().As(globalConfig); err != nil {
		return false, errors.Wrap(err, "get config")
	}

	config := globalConfig.Tinkoff
	if config.Show {
		creds, err := DecodeCredentialsFrom(config.Data)
		if err != nil {
			return false, errors.Wrap(err, "decode credentials")
		}

		separator := "------------"
		println(separator)
		for key, cred := range creds {
			fmt.Printf("id: %d\nusername: %s\npassword: %s\n%s\n", key, cred.Username, cred.Password, separator)
		}

		return true, nil
	}

	if config.Generate {
		return true, errors.Wrap(config.Credentials.EncodeTo(config.Data), "generate")
	}

	return false, nil
}
