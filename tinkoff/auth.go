package tinkoff

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tebeka/selenium"
	"golang.org/x/crypto/ssh/terminal"
)

type Auth interface {
	SessionID() (string, error)
}

type SessionID string

func (id SessionID) SessionID() (string, error) {
	return string(id), nil
}

type WebAuth struct {
	WebDriverConfig
	TwoFactor
	Username  string
	Password  string
	sessionID string
}

func (a *WebAuth) SessionID() (string, error) {
	if a.sessionID != "" {
		return a.sessionID, nil
	}

	wd, err := NewChromeDriver(a.WebDriverConfig)
	if err != nil {
		panic(err)
	}
	defer wd.Close()

	if err := wd.Get(LoginPage); err != nil {
		return "", errors.Wrap(err, "get login page")
	}

	if a.Username == "" {
		if a.Username, err = a.RequestCode("username", ""); err != nil {
			return "", errors.Wrap(err, "request username")
		}

		a.Username = strings.Trim(a.Username, " \n")
	}

	if err := wd.FindByNameAnd("login", func(e selenium.WebElement) error {
		return e.SendKeys(a.Username)
	}); err != nil {
		return "", errors.Wrap(err, "enter login")
	}

	password := a.Password
	if password == "" {
		if password, err = a.RequestCode("password", a.Username); err != nil {
			return "", errors.Wrap(err, "request password")
		}
	}

	if err := wd.FindByNameAnd("password", func(e selenium.WebElement) error {
		return e.SendKeys(password + selenium.EnterKey)
	}); err != nil {
		return "", errors.Wrap(err, "enter password")
	}

	codeValue, err := a.RequestCode("SMS code", a.Username)
	if err != nil {
		return "", errors.Wrap(err, "request code")
	}

	if err := wd.FindByNameAnd("code", func(e selenium.WebElement) error {
		return e.SendKeys(codeValue + selenium.EnterKey)
	}); err != nil {
		return "", errors.Wrap(err, "enter code")
	}

	// language=XPath
	blackXPath := `//*[text() = 'Счет Tinkoff Black']`
	if _, err := wd.FindElement(selenium.ByXPATH, blackXPath); err != nil {
		return "", errors.Wrap(err, "find black card")
	}

	var psid selenium.Cookie
	for {
		psid, err = wd.GetCookie("psid")
		if err != nil {
			return "", errors.Wrap(err, "get session ID")
		}

		if psid.Value != "" {
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	a.sessionID = psid.Value
	log.Printf("Received session ID for Tinkoff: %s", a.sessionID)
	return a.sessionID, nil
}

type TwoFactor interface {
	RequestCode(description, username string) (string, error)
}

var BasicTwoFactor TwoFactor = basicTwoFactor{}

type basicTwoFactor struct{}

func (f basicTwoFactor) RequestCode(description, username string) (string, error) {
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
