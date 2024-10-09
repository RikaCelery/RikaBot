package utils

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
	"html/template"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type ScreenShotPageOption struct {
	Width    int
	Height   int
	DPI      float64
	Before   func(page playwright.Page)
	PwOption playwright.PageScreenshotOptions
	Sleep    time.Duration
}
type ScreenShotElementOption struct {
	Width    int
	Height   int
	DPI      float64
	Before   func(page playwright.Page)
	PwOption playwright.ElementHandleScreenshotOptions
	Sleep    time.Duration
}

var (
	pw                 *playwright.Playwright
	ctx                playwright.BrowserContext
	inited             = false
	defaultPageOptions = playwright.PageScreenshotOptions{
		FullPage:   playwright.Bool(true),
		Type:       playwright.ScreenshotTypeJpeg,
		Quality:    playwright.Int(70),
		Timeout:    playwright.Float(60_000),
		Animations: playwright.ScreenshotAnimationsAllow,
		Scale:      playwright.ScreenshotScaleDevice,
		Style:      playwright.String(`body{padding: 0;margin: 0;}`),
	}
	defaultElementOptions = playwright.ElementHandleScreenshotOptions{
		Type:       playwright.ScreenshotTypeJpeg,
		Quality:    playwright.Int(70),
		Timeout:    playwright.Float(60_000),
		Animations: playwright.ScreenshotAnimationsAllow,
		Scale:      playwright.ScreenshotScaleDevice,
		Style:      playwright.String(`body{padding: 0;margin: 0;}`),
	}
	globalWaiter = 1
)

func init() {
	var err error
	pw, err = playwright.Run()
	if err != nil {
		err := playwright.Install()
		if err != nil {
			return
		}
		pw, err = playwright.Run()
		if err != nil {
			return
		}
	}

	ctx, err = pw.Chromium.LaunchPersistentContext("./bw", playwright.BrowserTypeLaunchPersistentContextOptions{
		DeviceScaleFactor: playwright.Float(1.5),
		ChromiumSandbox:   playwright.Bool(false),
		AcceptDownloads:   playwright.Bool(false),
		Headless:          playwright.Bool(true),
	})
	if err != nil {
		return
	}
	inited = true

}

// ScreenShotPageURL 截屏
func ScreenShotPageURL(u string, option ...ScreenShotPageOption) (bytes []byte, err error) {
	if !inited {
		return nil, errors.New("playwright not inited")
	}
	parse, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	ipReg := regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`)
	if parse.Scheme == "" {
		if ipReg.MatchString(parse.Host) {
			parse.Scheme = "http"
		} else {
			parse.Scheme = "https"
		}
	}
	if parse.Scheme != "https" && parse.Scheme != "http" {
		return nil, errors.New("unsupport schema")
	}
	o := ScreenShotPageOption{Width: 600,
		Sleep: time.Millisecond * 100, PwOption: defaultPageOptions}
	if len(option) != 0 {
		o = option[0]
	}
	ctx := ctx
	if o.DPI != 0 {
		ctx, err = pw.Chromium.LaunchPersistentContext(fmt.Sprintf("./bw%.1f", o.DPI), playwright.BrowserTypeLaunchPersistentContextOptions{
			DeviceScaleFactor: playwright.Float(o.DPI),
			ChromiumSandbox:   playwright.Bool(false),
			AcceptDownloads:   playwright.Bool(false),
			Headless:          playwright.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		defer ctx.Close()
	}
	page, err := ctx.NewPage()
	if err != nil {
		return nil, err
	}
	defer page.Close()
	response, err := page.Goto(parse.String())
	if err != nil {
		return nil, err
	}
	if !response.Ok() {
		return nil, errors.New("response not ok")
	}
	if o.Height == 0 {
		o.Height = 100
	}
	err = page.SetViewportSize(o.Width, o.Height)
	if err != nil {
		return nil, err
	}
	if o.Before != nil {
		o.Before(page)
	}
	time.Sleep(o.Sleep)
	screenshot, err := page.Screenshot(o.PwOption)
	if err != nil {
		return nil, err
	}
	return screenshot, err
}

func ScreenShotPageContent(content string, option ...ScreenShotPageOption) (bytes []byte, err error) {
	if !inited {
		return nil, errors.New("playwright not inited")
	}
	o := ScreenShotPageOption{Width: 600,
		Sleep: time.Millisecond * 100, PwOption: defaultPageOptions}
	if len(option) != 0 {
		o = option[0]
	}
	ctx := ctx
	if o.DPI != 0 {
		ctx, err = pw.Chromium.LaunchPersistentContext(fmt.Sprintf("./bw%.1f", o.DPI), playwright.BrowserTypeLaunchPersistentContextOptions{
			DeviceScaleFactor: playwright.Float(o.DPI),
			ChromiumSandbox:   playwright.Bool(false),
			AcceptDownloads:   playwright.Bool(false),
			Headless:          playwright.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		defer ctx.Close()
	}
	page, err := ctx.NewPage()
	if err != nil {
		return nil, err
	}
	defer page.Close()
	err = page.SetContent(content)
	if err != nil {
		return nil, err
	}
	if o.Height == 0 {
		o.Height = 100
	}
	err = page.SetViewportSize(o.Width, o.Height)
	if err != nil {
		return nil, err
	}
	if o.Before != nil {
		o.Before(page)
	}
	time.Sleep(o.Sleep)
	screenshot, err := page.Screenshot(o.PwOption)
	if err != nil {
		return nil, err
	}
	return screenshot, err
}
func ScreenShotElementContent(content string, selector string, option ...ScreenShotElementOption) (bytes []byte, err error) {
	if !inited {
		return nil, errors.New("playwright not inited")
	}
	ctx := ctx
	o := ScreenShotElementOption{Width: 600,
		Sleep: time.Millisecond * 100, PwOption: defaultElementOptions}
	if len(option) != 0 {
		o = option[0]
	}
	if o.DPI != 0 {
		ctx, err = pw.Chromium.LaunchPersistentContext(fmt.Sprintf("./bw%.1f", o.DPI), playwright.BrowserTypeLaunchPersistentContextOptions{
			DeviceScaleFactor: playwright.Float(o.DPI),
			ChromiumSandbox:   playwright.Bool(false),
			AcceptDownloads:   playwright.Bool(false),
			Headless:          playwright.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		defer ctx.Close()
	}
	page, err := ctx.NewPage()
	if err != nil {
		return nil, err
	}
	defer page.Close()
	err = page.SetContent(content)
	if err != nil {
		return nil, err
	}
	if o.Height == 0 {
		o.Height = 100
	}
	err = page.SetViewportSize(o.Width, o.Height)
	if err != nil {
		return nil, err
	}
	if o.Before != nil {
		o.Before(page)
	}
	time.Sleep(o.Sleep)
	querySelector, err := page.QuerySelector(selector)
	if err != nil {
		return nil, err
	}
	querySelector.ScrollIntoViewIfNeeded()
	screenshot, err := querySelector.Screenshot(o.PwOption)
	return screenshot, err
}

func ScreenShotPageTemplate(name string, data any, option ...ScreenShotPageOption) (bytes []byte, err error) {
	if !inited {
		return nil, errors.New("playwright not inited")
	}
	funcs := template.FuncMap{
		"truncate": Truncate,
		"replace": func(src string, reg string, repl string) string {
			regex, err := regexp.Compile(reg)
			if err != nil {
				logrus.Errorf("regexp compile error: %v", err)
				panic(err)
			}
			return regex.ReplaceAllString(src, repl)
		},
		"escape": func(in string) template.HTML {
			return template.HTML(in)
		},
		"tohtml": func(in string) *goquery.Document {
			reader, err := goquery.NewDocumentFromReader(strings.NewReader(in))
			if err != nil {
				logrus.Errorf("tohtml error: %v", err)
				panic(err)
			}
			return reader
		},
		"select": func(in string, selector string) *goquery.Selection {
			reader, err := goquery.NewDocumentFromReader(strings.NewReader(in))
			if err != nil {
				logrus.Errorf("select error: %v", err)
				panic(err)
			}
			find := reader.Find(selector)
			// println(in, selector, find)
			return find
		},
		"selContent": func(in *goquery.Selection) template.HTML {
			return template.HTML(strings.Trim(in.Text(), " \n\r"))
		},
		"docContent": func(in *goquery.Document) template.HTML {
			return template.HTML(strings.Trim(in.Text(), " \n\r"))
		},
		"startWith": strings.HasPrefix,
		"endWith":   strings.HasSuffix,
		"isnil": func(obj interface{}) bool {
			return obj == nil
		},
		"notnil": func(obj interface{}) bool {
			return obj != nil
		},
	}

	t, err := template.New(name).Funcs(funcs).ParseGlob("template/*")
	if err != nil {
		return nil, err
	}
	buf := strings.Builder{}
	err = t.Execute(&buf, data)
	if err != nil {
		return nil, err
	}
	return ScreenShotPageContent(buf.String(), option...)
}
func ScreenShotElementTemplate(name string, selector string, data any, option ...ScreenShotElementOption) (bytes []byte, err error) {
	if !inited {
		return nil, errors.New("playwright not inited")
	}
	funcs := template.FuncMap{
		"truncate": Truncate,
		"replace": func(src string, reg string, repl string) string {
			regex, err := regexp.Compile(reg)
			if err != nil {
				logrus.Errorf("regexp compile error: %v", err)
				panic(err)
			}
			return regex.ReplaceAllString(src, repl)
		},
		"escape": func(in string) template.HTML {
			return template.HTML(in)
		},
		"tohtml": func(in string) *goquery.Document {
			reader, err := goquery.NewDocumentFromReader(strings.NewReader(in))
			if err != nil {
				logrus.Errorf("tohtml error: %v", err)
				panic(err)
			}
			return reader
		},
		"select": func(in string, selector string) *goquery.Selection {
			reader, err := goquery.NewDocumentFromReader(strings.NewReader(in))
			if err != nil {
				logrus.Errorf("select error: %v", err)
				panic(err)
			}
			find := reader.Find(selector)
			// println(in, selector, find)
			return find
		},
		"selContent": func(in *goquery.Selection) template.HTML {
			return template.HTML(strings.Trim(in.Text(), " \n\r"))
		},
		"docContent": func(in *goquery.Document) template.HTML {
			return template.HTML(strings.Trim(in.Text(), " \n\r"))
		},
		"startWith": strings.HasPrefix,
		"endWith":   strings.HasSuffix,
		"isnil": func(obj interface{}) bool {
			return obj == nil
		},
		"notnil": func(obj interface{}) bool {
			return obj != nil
		},
	}

	t, err := template.New(name).Funcs(funcs).ParseGlob("template/*")
	if err != nil {
		return nil, err
	}
	buf := strings.Builder{}
	err = t.Execute(&buf, data)
	if err != nil {
		return nil, err
	}
	return ScreenShotElementContent(buf.String(), selector, option...)
}
