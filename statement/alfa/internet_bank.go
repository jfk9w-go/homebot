package alfa

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jfk9w-go/bank-statement/statement"
	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/csv"
	"github.com/pkg/errors"
	"github.com/tebeka/selenium"
	"golang.org/x/text/encoding/charmap"
)

var (
	LoginPage = "https://click.alfabank.ru"
)

type InternetBankConfig struct {
	// Username used to login via LoginPage.
	Username string `yaml:"username"`
	// Password used to login via LoginPage.
	Password string `yaml:"password"`
	// Accounts is a list of account IDs
	// (can be copied from "Счета" tab).
	Accounts []string `yaml:"accounts"`
}

type InternetBank struct {
	Config InternetBankConfig
}

func (b InternetBank) ID() string {
	return "Alfa"
}

func (b InternetBank) authorize(driver *statement.WebDriver, auth statement.TwoFactorAuthentication) error {
	if err := driver.Get(LoginPage); err != nil {
		return errors.Wrap(err, "get login page")
	}

	login, err := driver.FindElement(selenium.ByID, "login-input")
	if err != nil {
		return errors.Wrap(err, "find login input")
	}

	if err := login.SendKeys(b.Config.Username + selenium.EnterKey); err != nil {
		return errors.Wrap(err, "enter username")
	}

	password, err := driver.FindElement(selenium.ByID, "password-input")
	if err != nil {
		return errors.Wrap(err, "find password input")
	}

	if err := password.SendKeys(b.Config.Password + selenium.EnterKey); err != nil {
		return errors.Wrap(err, "enter password")
	}

	// language=XPath
	codeXPath := `//input[@type = 'tel']`
	code, err := driver.FindElement(selenium.ByXPATH, codeXPath)
	if err != nil {
		return errors.Wrap(err, "find code input")
	}

	codeValue, err := auth.RequestCode(b.ID(), b.Config.Username)
	if err != nil {
		return errors.Wrap(err, "request code")
	}

	if err := code.SendKeys(codeValue + selenium.EnterKey); err != nil {
		return errors.Wrap(err, "enter code")
	}

	return nil
}

func (b InternetBank) navigateToAccounts(driver *statement.WebDriver) error {
	// language=XPath
	xpath := `//span[text() = 'Счета']`
	return driver.FindAndClick(xpath)
}

func (b InternetBank) downloadAccountStatement(ctx context.Context, driver *statement.WebDriver, out chan<- *statement.BankStatement, account string) error {
	log.Printf("Downloading statement for %s account %s", b.ID(), account)

	// language=XPath
	xpath := fmt.Sprintf(`//span[text() = '%s']`, account)
	if err := driver.FindAndClick(xpath); err != nil {
		return errors.Wrap(err, "click account")
	}

	// language=XPath
	xpath = `//a[text() = 'Выписка по счёту']`
	if err := driver.FindAndClick(xpath); err != nil {
		return errors.Wrap(err, "click statement")
	}

	// language=XPath
	xpath = `//a[text() = 'два года']`
	if err := driver.FindAndClick(xpath); err != nil {
		return errors.Wrap(err, "click two years")
	}

	// language=XPath
	xpath = `//button[text() = 'Показать']`
	if err := driver.FindAndClick(xpath); err != nil {
		return errors.Wrap(err, "click show button")
	}

	// language=XPath
	xpath = `//a[@id = 'pt1:downloadCSVLink']`
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	file, err := driver.ExpectDownload(ctx, func() error { return driver.FindAndClick(xpath) }, 20*time.Second)
	if err != nil {
		return errors.Wrap(err, "download csv")
	}

	return flu.DecodeFrom(file, &csv.Codec{
		Output: RowOutput{
			Context:  ctx,
			Out:      out,
			BankID:   b.ID(),
			Username: b.Config.Username,
		},
		Comma:    ';',
		Encoding: charmap.Windows1251,
	})
}

func (b InternetBank) DownloadStatement(ctx context.Context, driver *statement.WebDriver, auth statement.TwoFactorAuthentication, out chan<- *statement.BankStatement) error {
	if err := b.authorize(driver, auth); err != nil {
		return errors.Wrap(err, "authorize")
	}

	for _, account := range b.Config.Accounts {
		if err := b.navigateToAccounts(driver); err != nil {
			return errors.Wrap(err, "navigate to accounts")
		}

		if err := b.downloadAccountStatement(ctx, driver, out, account); err != nil {
			return errors.Wrapf(err, "download account %s statement", account)
		}
	}

	return nil
}
