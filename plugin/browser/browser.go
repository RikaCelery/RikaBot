// Package browser playwright浏览器相关
package browser

import (
	"fmt"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/alexflint/go-arg"
	"github.com/playwright-community/playwright-go"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/shell"
	"github.com/wdvxdr1123/ZeroBot/message"
	"strings"

	"github.com/FloatTech/ZeroBot-Plugin/utils"
)

func init() {
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "浏览器",
		Help:             "- 截图 <网址>",
	})
	type cmd struct {
		Width   int      `arg:"-W" default:"600"`
		Height  int      `arg:"-H" help:"截图高度，0表示全屏" default:"0"`
		DPI     float64  `arg:"--dpi" default:"1.5"`
		Quality int      `arg:"-q" help:"截图质量，最高100" default:"30"`
		URL     []string `arg:"positional"`
	}
	engine.OnCommand("截图", zero.SuperUserPermission, func(ctx *zero.Ctx) bool {
		var screenShotCmd = cmd{}
		browserArgsParser, err := arg.NewParser(arg.Config{Program: zero.BotConfig.CommandPrefix + "截图", IgnoreEnv: true}, &screenShotCmd)
		if err != nil {
			panic(err)
		}
		err = browserArgsParser.Parse(shell.Parse(ctx.State["args"].(string)))
		if err != nil || len(screenShotCmd.URL) == 0 {
			buf := strings.Builder{}
			browserArgsParser.WriteHelp(&buf)
			ctx.Send(buf.String())
			ctx.Break()
			return false
		}

		ctx.State["flag"] = screenShotCmd
		return true
	}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		model := ctx.State["flag"].(cmd)
		option := utils.ScreenShotPageOption{
			Width: model.Width,
			DPI:   model.DPI,
			PwOption: playwright.PageScreenshotOptions{
				FullPage:   utils.DefaultPageOptions.FullPage,
				Type:       utils.DefaultPageOptions.Type,
				Quality:    playwright.Int(model.Quality),
				Timeout:    utils.DefaultPageOptions.Timeout,
				Animations: playwright.ScreenshotAnimationsAllow,
				Scale:      utils.DefaultPageOptions.Scale,
				Style:      utils.DefaultPageOptions.Style,
			},
		}
		if model.Height != 0 {
			option.Height = model.Height
			option.PwOption.FullPage = playwright.Bool(false)
		}
		for _, u := range model.URL {
			img, err := utils.ScreenShotPageURL(u, option)
			if err != nil {
				ctx.Send(fmt.Sprintf("ERROR: %v", err))
				return
			}
			ctx.Send(message.ImageBytes(img))
		}
	})
}
