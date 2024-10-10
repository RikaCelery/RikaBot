package utils

import (
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ScreenShotPageOption 截屏选项
type ScreenShotPageOption struct {
	Width    int
	Height   int
	DPI      float64
	Before   func(page playwright.Page)
	PwOption playwright.PageScreenshotOptions
	Sleep    time.Duration
}

// ScreenShotElementOption 元素截屏选项
type ScreenShotElementOption struct {
	Width    int
	Height   int
	DPI      float64
	Before   func(page playwright.Page)
	PwOption playwright.LocatorScreenshotOptions
	Sleep    time.Duration
}

var (
	pw     *playwright.Playwright
	ctx    playwright.BrowserContext
	inited = false
	// DefaultPageOptions 默认截图选项
	DefaultPageOptions = playwright.PageScreenshotOptions{
		FullPage:   playwright.Bool(true),
		Type:       playwright.ScreenshotTypeJpeg,
		Quality:    playwright.Int(70),
		Timeout:    playwright.Float(60_000),
		Animations: playwright.ScreenshotAnimationsAllow,
		Scale:      playwright.ScreenshotScaleDevice,
		Style:      playwright.String(`body{padding: 0;margin: 0;}`),
	}
	// DefaultElementOptions 默认元素截屏选项
	DefaultElementOptions = playwright.LocatorScreenshotOptions{
		Type:       playwright.ScreenshotTypeJpeg,
		Quality:    playwright.Int(70),
		Timeout:    playwright.Float(60_000),
		Animations: playwright.ScreenshotAnimationsAllow,
		Scale:      playwright.ScreenshotScaleDevice,
		Style:      playwright.String(`body{padding: 0;margin: 0;}`),
	}
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
		//ColorScheme:       playwright.ColorSchemeDark,
	})
	if err != nil {
		return
	}
	inited = true
}

// WaitImage 等待图片加载完毕
func WaitImage(page playwright.Page) {
	all, _ := page.Locator("img").All()
	for _, locator := range all {
		if visible, _ := locator.IsVisible(); !visible {
			continue
		}
		_ = locator.ScrollIntoViewIfNeeded()
		_ = playwright.NewPlaywrightAssertions().Locator(locator).ToHaveJSProperty("complete", true)
		_ = playwright.NewPlaywrightAssertions().Locator(locator).Not().ToHaveJSProperty("naturalWidth", 0)
	}
}

// ScreenShotPageURL 网址截屏
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
		Sleep: time.Millisecond * 100, PwOption: DefaultPageOptions}
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
	evaluated, err := page.Evaluate(`document.documentElement.scrollHeight`)
	o.Height = evaluated.(int)
	err = page.SetViewportSize(o.Width, o.Height)
	if err != nil {
		return nil, err
	}
	WaitImage(page)
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

// ScreenShotElementURL 网址元素截屏
func ScreenShotElementURL(u string, selector string, option ...ScreenShotElementOption) (bytes []byte, err error) {
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
	o := ScreenShotElementOption{Width: 600,
		Sleep: time.Millisecond * 100, PwOption: playwright.LocatorScreenshotOptions{}}
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
			//ColorScheme:       playwright.ColorSchemeDark,
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
	evaluated, err := page.Evaluate(`document.documentElement.scrollHeight`)
	o.Height = evaluated.(int)
	err = page.SetViewportSize(o.Width, o.Height)
	if err != nil {
		return nil, err
	}
	WaitImage(page)
	if o.Before != nil {
		o.Before(page)
	}
	time.Sleep(o.Sleep)
	locator := page.Locator(selector)
	return locator.Screenshot(o.PwOption)
}

// ScreenShotPageContent 自定义内容截屏
func ScreenShotPageContent(content string, option ...ScreenShotPageOption) (bytes []byte, err error) {
	if !inited {
		return nil, errors.New("playwright not inited")
	}
	o := ScreenShotPageOption{Width: 600,
		Sleep: time.Millisecond * 100, PwOption: DefaultPageOptions}
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
			//ColorScheme:       playwright.ColorSchemeDark,
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
	evaluated, err := page.Evaluate(`document.documentElement.scrollHeight`)
	o.Height = evaluated.(int)
	err = page.SetViewportSize(o.Width, o.Height)
	if err != nil {
		return nil, err
	}
	WaitImage(page)
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

// ScreenShotElementContent 自定义元素截屏
func ScreenShotElementContent(content string, selector string, option ...ScreenShotElementOption) (bytes []byte, err error) {
	if !inited {
		return nil, errors.New("playwright not inited")
	}
	ctx := ctx
	o := ScreenShotElementOption{Width: 600,
		Sleep: time.Millisecond * 100, PwOption: DefaultElementOptions}
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
	evaluated, err := page.Evaluate(`document.documentElement.scrollHeight`)
	o.Height = evaluated.(int)
	err = page.SetViewportSize(o.Width, o.Height)
	if err != nil {
		return nil, err
	}
	WaitImage(page)
	if o.Before != nil {
		o.Before(page)
	}
	time.Sleep(o.Sleep)
	locator := page.Locator(selector)
	_ = locator.ScrollIntoViewIfNeeded(playwright.LocatorScrollIntoViewIfNeededOptions{Timeout: playwright.Float(1000)})
	screenshot, err := locator.Screenshot(o.PwOption)
	return screenshot, err
}

// ScreenShotPageTemplate 模板截屏
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

// ScreenShotElementTemplate 元素模板截屏
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
