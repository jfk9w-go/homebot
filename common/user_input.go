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

var BasicUserInput UserInput = basicUserInput{}

type basicUserInput struct{}

func (f basicUserInput) Request(description, username string) (string, error) {
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
