// Package previewer a plugin to generate preview images
package previewer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/FloatTech/floatbox/binary"
	"github.com/FloatTech/floatbox/web"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/ZeroBot-Plugin/utils"
)

type generator struct {
	name string
	gen  func(matched []string) ([]byte, error)
}

var mappers = make(map[*regexp.Regexp]generator)

func init() {
	e := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "生成预览",
		Help: `回自动识别链接消息并生成预览图片
注：你需要向Bot妈申请权限, 你可以使用/report功能来向Bot妈申请
目前支持：
X(Twitter): 用户的推文（回复的评论不算）

`,
	})
	mappers[regexp.MustCompile(`(?:x|twitter)\.com/(\w+)/status/(\d+)`)] = generator{
		name: "twitter",
		gen: func(matched []string) ([]byte, error) {
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
		},
	}
	mappers[regexp.MustCompile(`wx9e14317d7c5d2267.h5.qiqiya.cc/wx9e14317d7c5d2267/wall/post/(\d+)`)] = generator{
		name: "treehole",
		gen: func(matched []string) ([]byte, error) {
			type PostBasic struct {
				CommentsCount int    `json:"commentsCount"`
				Contents      string `json:"contents"`
				Gender        string `json:"gender"`
				ID            int    `json:"id"`
				SendTime      int    `json:"sendTime"`
				SenderID      int    `json:"senderId"`
				SenderName    string `json:"senderName"`
				Views         int    `json:"views"`
			}
			data, err := web.GetDataRetry(fmt.Sprintf("http://localhost:6678/post?id=%s", matched[1]), 2)
			if err != nil {
				return nil, err
			}
			feed := utils.FromJSON[PostBasic](binary.BytesToString(data))
			re := regexp.MustCompile(`,https://img.qiqi.pro/x/[^,]+`)
			feed.Contents = re.ReplaceAllStringFunc(feed.Contents, func(s string) string {
				return fmt.Sprintf(`<img src="%s" />`, s[1:])
			})
			feed.Gender = strings.ToLower(feed.Gender)
			bytes, err := utils.ScreenShotPageTemplate("treehole.html", feed, utils.ScreenShotPageOption{
				Width:    500,
				Height:   0,
				DPI:      0,
				Before:   nil,
				PwOption: utils.DefaultPageOptions,
				Sleep:    0,
			})
			return bytes, err
		},
	}
	e.OnMessage(func(ctx *zero.Ctx) bool {
		for r, v := range mappers {
			if r.MatchString(ctx.Event.RawMessage) {
				ctx.State["matched"] = r.FindStringSubmatch(ctx.Event.RawMessage)
				ctx.State["name"] = v.name
				ctx.State["generator"] = v.gen
				return true
			}
		}
		return false
	}, func(ctx *zero.Ctx) bool {
		if ctx.State["name"].(string) == "treehole" {
			return true
		}
		return zero.SuperUserPermission(ctx)
	}).Handle(func(ctx *zero.Ctx) {
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
