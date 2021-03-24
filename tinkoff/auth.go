package tinkoff

import (
	"log"
	"strings"
	"time"

	"github.com/jfk9w-go/bank-statement/common"
	"github.com/pkg/errors"
	"github.com/tebeka/selenium"
)

type Auth interface {
	SessionID() (string, error)
}

type SessionID string

func (id SessionID) SessionID() (string, error) {
	return string(id), nil
}

type WebAuth struct {
	common.WebDriverConfig
	common.UserInput
	Username  string
	Password  string
	sessionID string
}

func (a *WebAuth) SessionID() (string, error) {
	if a.sessionID != "" {
		return a.sessionID, nil
	}

	wd, err := common.NewChromeDriver(a.WebDriverConfig)
	if err != nil {
		panic(err)
	}
	defer wd.Close()

	if err := wd.Get(LoginPage); err != nil {
		return "", errors.Wrap(err, "get login page")
	}

	if a.Username == "" {
		if a.Username, err = a.Request("username", ""); err != nil {
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
		if password, err = a.Request("password", a.Username); err != nil {
			return "", errors.Wrap(err, "request password")
		}
	}

	if err := wd.FindByNameAnd("password", func(e selenium.WebElement) error {
		return e.SendKeys(password + selenium.EnterKey)
	}); err != nil {
		return "", errors.Wrap(err, "enter password")
	}

	codeValue, err := a.Request("SMS code", a.Username)
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
