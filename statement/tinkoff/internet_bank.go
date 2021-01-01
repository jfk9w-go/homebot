package tinkoff

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/jfk9w-go/bank-statement/statement"
	"github.com/jfk9w-go/flu/csv"
	fluhttp "github.com/jfk9w-go/flu/http"
	"github.com/pkg/errors"
	"github.com/tebeka/selenium"
	"golang.org/x/text/encoding/charmap"
)

var (
	Host             = "https://www.tinkoff.ru"
	LoginPage        = Host + "/login/"
	ExportOperations = Host + "/api/common/v1/export_operations/"
)

type InternetBankConfig struct {
	// Username used to login via LoginPage.
	Username string
	// Password used to login via LoginPage.
	Password string
	// Accounts is a mapping of account names (chosen by user)
	// to account IDs (can be extracted from an account page URL via browser).
	Accounts map[string]string
}

type InternetBank struct {
	Config InternetBankConfig
	Client *fluhttp.Client
}

func (b InternetBank) getSessionID(driver *statement.WebDriver, auth statement.TwoFactorAuthentication) (string, error) {
	if err := driver.Get(LoginPage); err != nil {
		return "", errors.Wrap(err, "get login page")
	}

	login, err := driver.FindElement(selenium.ByName, "login")
	if err != nil {
		return "", errors.Wrap(err, "find login")
	}

	if err := login.SendKeys(b.Config.Username); err != nil {
		return "", errors.Wrap(err, "fill login")
	}

	password, err := driver.FindElement(selenium.ByName, "password")
	if err != nil {
		return "", errors.Wrap(err, "find password")
	}

	if err := password.SendKeys(b.Config.Password + selenium.EnterKey); err != nil {
		return "", errors.Wrap(err, "fill password")
	}

	code, err := driver.FindElement(selenium.ByName, "code")
	if err != nil {
		return "", errors.Wrap(err, "find code")
	}

	codeValue, err := auth.RequestCode(b.ID(), b.Config.Username)
	if err != nil {
		return "", errors.Wrap(err, "request code")
	}

	if err := code.SendKeys(codeValue + selenium.EnterKey); err != nil {
		return "", errors.Wrap(err, "fill code")
	}

	// language=XPath
	blackXPath := `//*[text() = 'Счет Tinkoff Black']`
	if _, err := driver.FindElement(selenium.ByXPATH, blackXPath); err != nil {
		return "", errors.Wrap(err, "find black card")
	}

	var psid selenium.Cookie
	for {
		psid, err = driver.GetCookie("psid")
		if err != nil {
			return "", errors.Wrap(err, "get session ID")
		}

		if psid.Value != "" {
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	return psid.Value, nil
}

func (b InternetBank) ID() string {
	return "Tinkoff"
}

func (b InternetBank) DownloadStatement(
	ctx context.Context, driver *statement.WebDriver,
	auth statement.TwoFactorAuthentication, out chan<- *statement.BankStatement) error {

	sessionID, err := b.getSessionID(driver, auth)
	if err != nil {
		return errors.Wrap(err, "get session ID")
	}

	now := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
	for accountName, accountID := range b.Config.Accounts {
		log.Printf("Downloading statement for %s account %s (%s)", b.ID(), accountName, accountID)
		if err := b.Client.GET(ExportOperations).
			QueryParam("format", "csv").
			QueryParam("start", "0").
			QueryParam("end", now).
			QueryParam("account", accountID).
			QueryParam("card", "0").
			QueryParam("sessionid", sessionID).
			Context(ctx).
			Execute().
			DecodeBody(&csv.Codec{
				Output: RowOutput{
					Context:     ctx,
					Out:         out,
					BankID:      b.ID(),
					Username:    b.Config.Username,
					AccountName: accountName,
					AccountID:   accountID,
				},
				Comma:    ';',
				Encoding: charmap.Windows1251,
			}).Error; err != nil {
			return errors.Wrapf(err, "on account %s (%s)", accountName, accountID)
		}
	}

	return nil
}
