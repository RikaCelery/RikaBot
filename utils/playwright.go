package utils

import (
	"errors"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	log "github.com/sirupsen/logrus"
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
	// GlobalCSS 全局样式, 用于屏蔽浮动/广告元素等
	GlobalCSS = `
#ageDisclaimerMainBG,
.desktop-dialog-open,
#modalWrapMTubes {
    display: none !important;
    visibility: hidden !important;
}

body {
    padding: 0;
    margin: 0;
}
`
	pw *playwright.Playwright
	//ctx    playwright.BrowserContext
	inited = false
	// DefaultPageOptions 默认截图选项
	DefaultPageOptions = playwright.PageScreenshotOptions{
		FullPage:   playwright.Bool(true),
		Type:       playwright.ScreenshotTypeJpeg,
		Quality:    playwright.Int(70),
		Timeout:    playwright.Float(60_000),
		Animations: playwright.ScreenshotAnimationsAllow,
		Scale:      playwright.ScreenshotScaleDevice,
		Style:      playwright.String(GlobalCSS + "\n" + ``),
	}
	// DefaultElementOptions 默认元素截屏选项
	DefaultElementOptions = playwright.LocatorScreenshotOptions{
		Type:       playwright.ScreenshotTypeJpeg,
		Quality:    playwright.Int(70),
		Timeout:    playwright.Float(60_000),
		Animations: playwright.ScreenshotAnimationsAllow,
		Scale:      playwright.ScreenshotScaleDevice,
		Style:      playwright.String(GlobalCSS + "\n" + `body{padding: 0;margin: 0;}`),
	}
)

func init() {
	var err error
	pw, err = playwright.Run()
	if err != nil {
		err := playwright.Install()
		if err != nil {
			panic(err)
		}
		pw, err = playwright.Run()
		if err != nil {
			panic(err)
		}
	}

	//ctx, err = pw.Chromium.LaunchPersistentContext("./bw", playwright.BrowserTypeLaunchPersistentContextOptions{
	//	DeviceScaleFactor: playwright.Float(1.5),
	//	ChromiumSandbox:   playwright.Bool(false),
	//	AcceptDownloads:   playwright.Bool(false),
	//	Headless:          playwright.Bool(true),
	//	Proxy: &playwright.Proxy{
	//		Server:   "http://localhost:7890",
	//		Bypass:   nil,
	//		Username: nil,
	//		Password: nil,
	//	},
	//	//ColorScheme:       playwright.ColorSchemeDark,
	//})
	if err != nil {
		panic(err)
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
		_, _ = locator.Evaluate(`(e) => {
    if (e.offsetParent) {
        e.offsetParent.scrollLeft = 0;
        e.offsetParent.scrollTop = 0;
    }
}`, nil)
		_ = playwright.NewPlaywrightAssertions().Locator(locator).ToHaveJSProperty("complete", true)
		_ = playwright.NewPlaywrightAssertions().Locator(locator).Not().ToHaveJSProperty("naturalWidth", 0)
	}
	_, _ = page.Evaluate(`document.querySelectorAll("*").forEach(e=>{e.scrollLeft = 0;e.scrollTop = 0;})`)
}

// Clean 清理页面，移除浮动元素等
func Clean(page playwright.Page) {
	_, _ = page.Evaluate(`document.classList.remove("xh-thumb-disabled")
if (location.hostname.search(/pornhub/) !== -1) {
    document.body.classList.remove("isOpenMTubes")
    document.querySelectorAll('.emptyBlockSpace,.sniperModeEngaged,#welcome,.wrapper > header,body > div.networkBarWrapper').forEach(e => e.remove())
    const main = document.querySelector(".wrapper")
    let el = main.nextElementSibling
    while (el) {
        let _el = el.nextElementSibling
        el.remove()
        el = _el
    }

}
else if (location.hostname.search(/xvideos/!=1)){
    document.documentElement.classList.remove("img-blured")
    document.documentElement.classList.remove("disclaimer-opened")
    document.documentElement.classList.remove("notouch")
    document.querySelectorAll("#disclaimer_background").forEach(e=>e.remove())
    document.querySelectorAll(".head__menu-line").forEach(e=>e.remove())
    document.querySelectorAll("#main .search-premium-tabs").forEach(e=>e.remove())
    document.querySelectorAll("#cookies-use-alert").forEach(e=>e.remove())
    document.querySelectorAll("header .head__search").forEach(e=>e.remove())
    document.querySelectorAll("header #header-mobile-right").forEach(e=>e.remove())
    document.querySelectorAll(".premium-results-line").forEach(e=>e.remove())
    document.querySelectorAll("#ad-footer,.remove-ads,#footer").forEach(e=>e.remove())
    document.querySelectorAll("#content .pagination").forEach(e=>e.remove())
}
else if (location.hostname.search(/xhamster/!=1)){
    document.documentElement.style.setProperty("--top-menu-height",0)
    document.querySelectorAll("*").forEach(e=>{
        if(e.classList.toString().search(/cookiesAnnounce-\w+/)!=-1)
            e.remove()
    })
    const inj = document.createElement("style")
    inj.innerHTML = ".header .search-section{justify-content: center;}"
    document.head.appendChild(inj)
    document.querySelectorAll('[data-role="promo-messages-wrapper"],nav.top-menu-container,.categories-list .main-categories,.categories-list .sidebar-filter-container,.Cm-vapremium-n-overlay,footer').forEach(e=>e.remove())
    document.querySelectorAll('.login-section,.search-container,.lang-geo-picker-container').forEach(e=>e.remove())
}`)
}
func launchContext(dpi float64) (playwright.BrowserContext, error) {
	if dpi == 0 {
		dpi = 1.5
	}
	context, err := pw.Chromium.LaunchPersistentContext(fmt.Sprintf("./bw%.1f", dpi), playwright.BrowserTypeLaunchPersistentContextOptions{
		DeviceScaleFactor: playwright.Float(dpi),
		ChromiumSandbox:   playwright.Bool(false),
		AcceptDownloads:   playwright.Bool(false),
		Headless:          playwright.Bool(true),
		Proxy: &playwright.Proxy{
			Server:   "http://localhost:7890",
			Bypass:   nil,
			Username: playwright.String(os.Getenv("PROXY_USERNAME")),
			Password: playwright.String(os.Getenv("PROXY_PASSWORD")),
		},
	})
	if err == nil {
		_ = context.Route("https://*.qlogo.cn/**", func(route playwright.Route) {
			_ = route.Continue()
		})
	}
	return context, err
}
func getScreenShotPageOption(option []ScreenShotPageOption) ScreenShotPageOption {
	o := ScreenShotPageOption{Width: 600, Sleep: time.Millisecond * 100, PwOption: DefaultPageOptions}
	if len(option) != 0 {
		o = option[0]
	}
	return o
}

func getScreenShotElementOption(option []ScreenShotElementOption) ScreenShotElementOption {
	o := ScreenShotElementOption{Width: 600, Sleep: time.Millisecond * 100, PwOption: DefaultElementOptions}
	if len(option) != 0 {
		o = option[0]
	}
	return o
}

// ScreenShotPageContent takes a string of HTML content and optionally a set of screen shot options to capture a page screenshot.
// This function initializes the browser context, creates a new page, sets the page content, and then takes a screenshot according to the specified options.
func ScreenShotPageContent(content string, option ...ScreenShotPageOption) (bytes []byte, err error) {
	// Check if Playwright has been initialized
	if !inited {
		log.Error("Playwright not initialized")
		return nil, errors.New("playwright not inited")
	}

	// Get the screen shot options, using the default if none are provided
	o := getScreenShotPageOption(option)

	// Launch a new browser context
	ctx, err := launchContext(o.DPI)
	if err != nil {
		log.Errorf("Failed to launch context: %v", err)
		return nil, err
	}
	defer ctx.Close()

	// Create a new page in the browser context
	page, err := ctx.NewPage()
	if err != nil {
		log.Errorf("Failed to create page: %v", err)
		return nil, err
	}
	defer page.Close()

	// Set the content of the page
	err = page.SetContent(content)
	if err != nil {
		log.Errorf("Failed to set page content: %v", err)
		return nil, err
	}

	// Take a screenshot of the page with the specified options
	return screenShotPage(page, o.Width, o.Height, o.Sleep, o.Before, o.PwOption)
}

// ScreenShotElementContent takes a string of HTML content, a CSS selector for the target element, and optionally a set of screen shot options to capture a screenshot of a specific element.
// This function initializes the browser context, creates a new page, sets the page content, and then takes a screenshot of the specified element according to the options.
func ScreenShotElementContent(content string, selector string, option ...ScreenShotElementOption) (bytes []byte, err error) {
	// Check if Playwright has been initialized
	if !inited {
		log.Error("Playwright not initialized")
		return nil, errors.New("playwright not inited")
	}

	// Get the screen shot options for the element, using the default if none are provided
	o := getScreenShotElementOption(option)

	// Launch a new browser context
	ctx, err := launchContext(o.DPI)
	if err != nil {
		log.Errorf("Failed to launch context: %v", err)
		return nil, err
	}
	defer ctx.Close()

	// Create a new page in the browser context
	page, err := ctx.NewPage()
	if err != nil {
		log.Errorf("Failed to create page: %v", err)
		return nil, err
	}
	defer page.Close()

	// Set the content of the page
	err = page.SetContent(content)
	if err != nil {
		log.Errorf("Failed to set page content: %v", err)
		return nil, err
	}

	// Log the start of taking a screenshot
	log.Info("Taking screenshot")

	// Take a screenshot of the specified element with the specified options
	return screenShotElement(page, selector, o.Width, o.Height, o.Sleep, o.Before, o.PwOption)
}

func screenShotPage(page playwright.Page, width, height int, sleep time.Duration, before func(page playwright.Page), pwOption playwright.PageScreenshotOptions) ([]byte, error) {
	Clean(page)

	if height == 0 {
		height = 100
	}

	err := page.SetViewportSize(width, height)
	if err != nil {
		log.Printf("Error setting viewport size: %v", err)
		return nil, err
	}

	evaluated, err := page.Evaluate(`document.documentElement.scrollHeight`)
	if err != nil {
		log.Printf("Error evaluating document scroll height: %v", err)
		return nil, err
	}

	height = evaluated.(int)

	err = page.SetViewportSize(width, height)
	if err != nil {
		log.Printf("Error setting viewport size after evaluation: %v", err)
		return nil, err
	}

	WaitImage(page)

	if before != nil {
		before(page)
	}

	time.Sleep(sleep)

	screenshot, err := page.Screenshot(pwOption)
	if err != nil {
		log.Printf("Error taking screenshot: %v", err)
		return nil, err
	}

	return screenshot, nil
}

func screenShotElement(page playwright.Page, selector string, width, height int, sleep time.Duration, before func(page playwright.Page), pwOption playwright.LocatorScreenshotOptions) ([]byte, error) {
	logger := log.New()

	Clean(page)

	if height == 0 {
		height = 100
	}

	err := page.SetViewportSize(width, height)
	if err != nil {
		logger.WithError(err).Error("Error setting viewport size")
		return nil, err
	}

	evaluated, err := page.Evaluate(`document.documentElement.scrollHeight`)
	if err != nil {
		logger.WithError(err).Error("Error evaluating document scroll height")
		return nil, err
	}

	height = evaluated.(int)

	err = page.SetViewportSize(width, height)
	if err != nil {
		logger.WithError(err).Error("Error setting viewport size after evaluation")
		return nil, err
	}

	WaitImage(page)

	if before != nil {
		before(page)
	}

	time.Sleep(sleep)

	locator := page.Locator(selector)
	screenshot, err := locator.Screenshot(pwOption)
	if err != nil {
		logger.WithError(err).Error("Error taking screenshot")
		return nil, err
	}

	return screenshot, nil
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
	o := getScreenShotPageOption(option)
	ctx, err := launchContext(o.DPI)
	if err != nil {
		log.Errorf("Failed to launch context: %v", err)
		return nil, err
	}
	defer ctx.Close()
	page, err := ctx.NewPage()
	if err != nil {
		return nil, err
	}
	defer page.Close()
	response, err := page.Goto(parse.String())
	if errors.Is(err, playwright.ErrTimeout) {
		response, err = page.Goto(parse.String())
	}
	if err != nil {
		return nil, err
	}
	if !response.Ok() {
		return nil, errors.New("response not ok")
	}
	return screenShotPage(page, o.Width, o.Height, o.Sleep, o.Before, o.PwOption)
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
	o := getScreenShotElementOption(option)
	ctx, err := launchContext(o.DPI)
	if err != nil {
		log.Errorf("Failed to launch context: %v", err)
		return nil, err
	}
	defer ctx.Close()
	page, err := ctx.NewPage()
	if err != nil {
		return nil, err
	}
	defer page.Close()
	response, err := page.Goto(parse.String())
	if errors.Is(err, playwright.ErrTimeout) {
		response, err = page.Goto(parse.String())
	}
	if err != nil {
		return nil, err
	}
	if !response.Ok() {
		return nil, errors.New("response not ok")
	}
	return screenShotElement(page, selector, o.Width, o.Height, o.Sleep, o.Before, o.PwOption)
}

var funcs = template.FuncMap{
	"truncate": Truncate,
	"escape": func(in string) template.HTML {
		return template.HTML(in)
	},
	"replace": func(src string, reg string, repl string) string {
		regex, err := regexp.Compile(reg)
		if err != nil {
			log.Errorf("regexp compile error: %v", err)
			return src // 返回原始字符串
		}
		return regex.ReplaceAllString(src, repl)
	},
	"tohtml": func(in string) *goquery.Document {
		reader, err := goquery.NewDocumentFromReader(strings.NewReader(in))
		if err != nil {
			log.Errorf("tohtml error: %v", err)
			return nil // 返回空指针
		}
		return reader
	},
	"select": func(in string, selector string) *goquery.Selection {
		reader, err := goquery.NewDocumentFromReader(strings.NewReader(in))
		if err != nil {
			log.Errorf("select error: %v", err)
			return nil // 返回空指针
		}
		find := reader.Find(selector)
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

// ScreenShotPageTemplate 模板截屏
func ScreenShotPageTemplate(name string, data any, option ...ScreenShotPageOption) (bytes []byte, err error) {
	if !inited {
		return nil, errors.New("playwright not inited")
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
