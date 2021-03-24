package common

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/phayes/freeport"
	"github.com/pkg/errors"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

type WebDriverConfig struct {
	SeleniumPath string
	DriverPath   string
	WaitTimeout  time.Duration
}

type WebDriver struct {
	selenium.WebDriver
	service    *selenium.Service
	tempDir    string
	downloadMu sync.Mutex
}

func NewChromeDriver(config WebDriverConfig) (*WebDriver, error) {
	port, err := freeport.GetFreePort()
	if err != nil {
		return nil, errors.Wrap(err, "find free port")
	}

	service, err := selenium.NewSeleniumService(config.SeleniumPath, port,
		selenium.ChromeDriver(config.DriverPath),
		selenium.Output(nil))
	if err != nil {
		return nil, errors.Wrap(err, "new selenium")
	}

	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("web-driver-%d", time.Now().Nanosecond()))
	_ = os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		return nil, errors.Wrapf(err, "create temp dir %s", tempDir)
	}

	caps := selenium.Capabilities{
		"browserName": "chrome",
		chrome.CapabilitiesKey: chrome.Capabilities{
			Args: []string{"--headless", "--window-size=1920,1080"},
			Prefs: map[string]interface{}{
				"download.default_directory":   tempDir,
				"download.prompt_for_download": false,
				"download.directory_upgrade":   true,
			},
		},
	}

	driver, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		return nil, errors.Wrap(err, "new remote")
	}

	if err := driver.SetImplicitWaitTimeout(config.WaitTimeout); err != nil {
		return nil, errors.Wrap(err, "set wait timeout")
	}

	return &WebDriver{
		WebDriver: driver,
		service:   service,
		tempDir:   tempDir,
	}, nil
}

func (wd *WebDriver) DownloadFile(ctx context.Context, action func() error, path string) error {
	wd.downloadMu.Lock()
	defer wd.downloadMu.Unlock()

	if err := action(); err != nil {
		return errors.Wrap(err, "download action")
	}

	var prevSize int64
	for {
		files, err := ioutil.ReadDir(wd.tempDir)
		if err != nil {
			return errors.Wrapf(err, "read %s directory", wd.tempDir)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			currSize := file.Size()
			if prevSize == 0 || prevSize != currSize {
				prevSize = currSize
				break
			} else {
				return os.Rename(filepath.Join(wd.tempDir, file.Name()), path)
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			continue
		}
	}
}

func (wd *WebDriver) FindByXPathAndClick(xpath string) error {
	return wd.FindByXPathAnd(xpath, selenium.WebElement.Click)
}

func (wd *WebDriver) FindByXPathAnd(xpath string, action func(selenium.WebElement) error) error {
	return wd.FindAnd(selenium.ByXPATH, xpath, action)
}

func (wd *WebDriver) FindByNameAnd(name string, action func(selenium.WebElement) error) error {
	return wd.FindAnd(selenium.ByName, name, action)
}

func (wd *WebDriver) FindAnd(by, value string, action func(selenium.WebElement) error) error {
	retry := 0
	for {
		element, err := wd.FindElement(by, value)
		if err == nil {
			if err = action(element); err == nil {
				return nil
			}
		}

		if retry < 5 {
			retry++
			time.Sleep(time.Duration(retry) * time.Second)
			continue
		}

		return errors.Wrapf(err, "with element %s", value)
	}
}

func (wd *WebDriver) Close() error {
	_ = wd.WebDriver.Close()
	_ = wd.service.Stop()
	_ = os.RemoveAll(wd.tempDir)
	return nil
}
