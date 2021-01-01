package statement

import (
	"bufio"
	"context"
	"fmt"
	"os"
)

type TwoFactorAuthentication interface {
	RequestCode(bankID, username string) (string, error)
}

type Bank interface {
	ID() string
	DownloadStatement(ctx context.Context, driver *WebDriver, auth TwoFactorAuthentication, out chan<- *BankStatement) error
}

type BasicTwoFactorAuthentication struct {
}

func (BasicTwoFactorAuthentication) RequestCode(bankID string, username string) (string, error) {
	fmt.Printf("Enter code for %s (%s): ", bankID, username)
	reader := bufio.NewReader(os.Stdin)
	return reader.ReadString('\n')
}
