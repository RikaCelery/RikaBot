package previewer

import (
	"fmt"
	"github.com/FloatTech/ZeroBot-Plugin/utils"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"regexp"
)

var mappers = make(map[*regexp.Regexp]func(matched []string) ([]byte, error))

func init() {
	e := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "生成预览",
		Help:             "回自动识别连接消息并生产预览",
	})
	mappers[regexp.MustCompile(`(?:x|twitter)\.com/(\w+)/status/(\d+)`)] = func(matched []string) ([]byte, error) {
		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(fmt.Sprintf("http://localhost:1200/twitter/tweet/%s/status/%s", matched[1], matched[2]))
		if err != nil {
			return nil, err
		}

		bytes, err := utils.ScreenShotPageTemplate("twitter.gohtml", feed, utils.ScreenShotPageOption{
			Width:    500,
			Height:   0,
			DPI:      0,
			Before:   nil,
			PwOption: utils.DefaultPageOptions,
			Sleep:    0,
		})
		return bytes, err
	}
	mappers[regexp.MustCompile(`steampowered\.com/app/(\d+)`)] = func(matched []string) ([]byte, error) {
		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(fmt.Sprintf("http://localhost:1200/twitter/tweet/%s/status/%s", matched[1], matched[2]))
		if err != nil {
			return nil, err
		}

		bytes, err := utils.ScreenShotPageTemplate("twitter.gohtml", feed, utils.ScreenShotPageOption{
			Width:    500,
			Height:   0,
			DPI:      0,
			Before:   nil,
			PwOption: utils.DefaultPageOptions,
			Sleep:    0,
		})
		return bytes, err
	}
	e.OnMessage(func(ctx *zero.Ctx) bool {
		for r, v := range mappers {
			if r.MatchString(ctx.Event.RawMessage) {
				ctx.State["matched"] = r.FindStringSubmatch(ctx.Event.RawMessage)
				ctx.State["generator"] = v
				return true
			}
		}
		return false
	}, zero.AdminPermission).Handle(func(ctx *zero.Ctx) {
		matched := ctx.State["matched"].([]string)
		generator := ctx.State["generator"].(func(matched []string) ([]byte, error))
		b, err := generator(matched)
		if err != nil {
			log.Error(err)
			return
		}
		ctx.Send(message.ImageBytes(b))
	})
}
