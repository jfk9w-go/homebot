package statement

import (
	"context"
	"os"

	"github.com/jfk9w-go/flu/serde"
	"github.com/phayes/freeport"

	"github.com/tebeka/selenium"

	fluhttp "github.com/jfk9w-go/flu/http"

	"github.com/pkg/errors"

	"github.com/jfk9w-go/flu"
)

type SeleniumConfig struct {
	// JarPath is a path to Selenium JAR file.
	JarPath string `yaml:"jar"`
	// ChromePath is a path to chromedriver binary.
	ChromePath string `yaml:"chrome"`
	// WaitTimeout is an implicit wait timeout for Selenium WebDriver.
	// Format is the same as in time.ParseDuration ("10s" for 10 seconds, for example).
	WaitTimeout serde.Duration `yaml:"wait_timeout"`
}

type Service struct {
	Client   *fluhttp.Client
	Auth     TwoFactorAuthentication
	Selenium SeleniumConfig
}

func (s *Service) UpdateStatement(ctx context.Context, banks []Bank, output BatchOutput) error {
	port, err := freeport.GetFreePort()
	if err != nil {
		return errors.Wrap(err, "find free port")
	}

	tempDir := os.TempDir()
	defer os.RemoveAll(tempDir)

	service, err := selenium.NewSeleniumService(s.Selenium.JarPath, port,
		selenium.ChromeDriver(s.Selenium.ChromePath),
		selenium.Output(nil))
	if err != nil {
		return errors.Wrap(err, "new selenium")
	}

	defer service.Stop()

	driver, err := NewChromeDriver(port, tempDir)
	if err != nil {
		return errors.Wrap(err, "new remote")
	}

	defer driver.Quit()

	if err := driver.SetImplicitWaitTimeout(s.Selenium.WaitTimeout.Duration); err != nil {
		return errors.Wrap(err, "set wait timeout")
	}

	ctx, cancel := context.WithCancel(ctx)

	var bg flu.WaitGroup
	defer bg.Wait()

	out := make(BankStatementIterable)
	defer close(out)

	bg.Go(ctx, nil, func(context.Context) {
		if err := output.Update(out); err != nil {
			cancel()
		}
	})

	for _, bank := range banks {
		if err := bank.DownloadStatement(ctx, driver, s.Auth, out); err != nil {
			out <- bankStatementCancel
			return errors.Wrapf(err, "bank %s", bank.ID())
		}
	}

	return nil
}
