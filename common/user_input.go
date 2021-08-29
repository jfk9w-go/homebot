package common

import (
	"bufio"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

type UserInput interface {
	Request(description, username string) (string, error)
}

var CLIUserInput UserInput = cliUserInput{}

type cliUserInput struct{}

func (f cliUserInput) Request(description, username string) (string, error) {
	if username != "" {
		username = " for " + username
	}

	fmt.Printf("Enter %s%s: ", description, username)
	reader := bufio.NewReader(os.Stdin)
	if username == "" {
		return reader.ReadString('\n')
	} else {
		data, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}

		return string(data), nil
	}
}

type PartialCLIUserInput map[string]string

func (in PartialCLIUserInput) Request(description, username string) (string, error) {
	value, ok := in[description]
	if ok {
		return value, nil
	}

	return CLIUserInput.Request(description, username)
}
