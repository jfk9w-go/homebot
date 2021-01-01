package statement

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tebeka/selenium/chrome"

	"github.com/jfk9w-go/flu"

	"github.com/pkg/errors"
	"github.com/tebeka/selenium"
)

type WebDriver struct {
	selenium.WebDriver
	downloadDir string
	downloadMu  sync.Mutex
}

func NewChromeDriver(port int, downloadDir string) (*WebDriver, error) {
	caps := selenium.Capabilities{
		"browserName": "chrome",
		chrome.CapabilitiesKey: chrome.Capabilities{
			Args: []string{"--headless", "--window-size=1920,1080"},
			Prefs: map[string]interface{}{
				"download.default_directory":   downloadDir,
				"download.prompt_for_download": false,
				"download.directory_upgrade":   true,
			},
		},
	}

	driver, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		return nil, errors.Wrap(err, "new remote")
	}

	return &WebDriver{
		WebDriver:   driver,
		downloadDir: downloadDir,
	}, nil
}

func (d *WebDriver) ExpectDownload(ctx context.Context, action func() error, timeout time.Duration) (flu.File, error) {
	d.downloadMu.Lock()
	defer d.downloadMu.Unlock()
	_ = os.RemoveAll(d.downloadDir)
	if err := os.MkdirAll(d.downloadDir, 0755); err != nil {
		return "", errors.Wrapf(err, "create %s directory", d.downloadDir)
	}

	if err := action(); err != nil {
		return "", errors.Wrap(err, "download action")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var prevSize int64
	for {
		files, err := ioutil.ReadDir(d.downloadDir)
		if err != nil {
			return "", errors.Wrapf(err, "read %s directory", d.downloadDir)
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
				return flu.File(filepath.Join(d.downloadDir, file.Name())), nil
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
			continue
		}
	}
}

func (d *WebDriver) FindAndClick(xpath string) error {
	return d.FindAnd(xpath, selenium.WebElement.Click)
}

func (d *WebDriver) FindAnd(xpath string, action func(selenium.WebElement) error) error {
	retry := 0
	for {
		element, err := d.FindElement(selenium.ByXPATH, xpath)
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

		return errors.Wrapf(err, "click element %s", xpath)
	}
}
