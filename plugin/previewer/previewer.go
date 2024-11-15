// Package previewer a plugin to generate preview images
package previewer

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FloatTech/ZeroBot-Plugin/utils"
	"github.com/FloatTech/floatbox/binary"
	"github.com/FloatTech/floatbox/web"
	"github.com/mmcdole/gofeed"
	"github.com/playwright-community/playwright-go"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	log "github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

type generator struct {
	name string
	gen  func(matched []string) ([]byte, error)
}

var mappers map[*regexp.Regexp]generator

var ErrBlocked = errors.New("该链接已被黑名单正则屏蔽")

type ShotConfig struct {
	Width   int     `json:"width"`
	Height  int     `json:"height"`
	DPI     float64 `json:"dpi"`
	Wait    int     `json:"wait"`
	Css     string  `json:"css"`
	Quality int     `json:"quality"`
}

type Config struct {
	Type                string     `json:"type"`
	Name                string     `json:"name"`
	Regex               string     `json:"regex"`
	BlacklistRegex      string     `json:"blacklist_regex"`
	MatchedGroup        int        `json:"matched_group"`
	UrlReplacementRegex string     `json:"url_replacement_regex"`
	UrlReplacement      string     `json:"url_replacement"`
	ErrorTemplate       string     `json:"error_template"`
	ScreenShotConfig    ShotConfig `json:"config"`
}

func initMapper(e *control.Engine) {
	mappers = make(map[*regexp.Regexp]generator)
	jb, err := os.ReadFile(path.Join(e.DataFolder(), "config.json"))
	if err != nil {
		goto builtin
	}
	{
		configs := make([]Config, 0)
		err := json.Unmarshal(jb, &configs)
		if err != nil {
			goto builtin
		}
		for _, v := range configs {
			log.Infoln("[previewer] 加载预览模板:", v.Name, v.Type)
			switch v.Type {
			case "SCREEN_SHOT":
				var replacer func(string) string
				var blacklist *regexp.Regexp
				if v.UrlReplacementRegex != "" {
					re, err := regexp.Compile(v.UrlReplacementRegex)
					if err != nil {
						log.Warnln("[previewer] 预览模板", v.Name, "url_replacement_regex 配置错误:", err)
						continue
					}
					replacer = func(s string) string {
						return re.ReplaceAllString(s, v.UrlReplacement)
					}
				}
				if v.BlacklistRegex != "" {
					blacklist, err = regexp.Compile(v.BlacklistRegex)
					if err != nil {
						log.Warnln("[previewer] 预览模板", v.Name, "blacklist_regex 配置错误:", err)
						continue
					}
				}
				mappers[regexp.MustCompile(v.Regex)] = generator{
					name: v.Name,
					gen: func(matched []string) ([]byte, error) {
						var url = matched[v.MatchedGroup]
						if replacer != nil {
							url = replacer(url)
						}
						if blacklist != nil && blacklist.MatchString(url) {
							return nil, ErrBlocked
						}
						opt := utils.DefaultPageOptions
						opt.Quality = playwright.Int(v.ScreenShotConfig.Quality)
						opt.Style = playwright.String(fmt.Sprintf("%s\n%s", *opt.Style, v.ScreenShotConfig.Css))
						bytes, err := utils.ScreenShotPageURL(url, utils.ScreenShotPageOption{
							Width:    v.ScreenShotConfig.Width,
							Height:   v.ScreenShotConfig.Height,
							DPI:      v.ScreenShotConfig.DPI,
							Sleep:    time.Duration(v.ScreenShotConfig.Wait) * time.Second,
							PwOption: opt,
						})
						if err != nil {
							return nil, err
						}
						return bytes, nil
					},
				}
			}
		}

	}
builtin:
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
	mappers[regexp.MustCompile(`https://www\.zhihu\.com/question/(\d+)/answer/(\d+)`)] = generator{
		name: "public-zhihu-answer",
		gen: func(matched []string) ([]byte, error) {
			fp := gofeed.NewParser()
			feed, err := fp.ParseURL(fmt.Sprintf("http://localhost:1200/zhihu/question/%s/answer/%s", matched[1], matched[2]))
			if err != nil {
				return nil, err
			}
			bytes, err := utils.ScreenShotPageTemplate("zhihu-answer.html", feed, utils.ScreenShotPageOption{
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
	mappers[regexp.MustCompile(`https://www\.zhihu\.com/question/(\d+)[^/]*`)] = generator{
		name: "public-zhihu-question",
		gen: func(matched []string) ([]byte, error) {
			fp := gofeed.NewParser()
			feed, err := fp.ParseURL(fmt.Sprintf("http://localhost:1200/zhihu/single-question/%s", matched[1]))
			if err != nil {
				return nil, err
			}
			bytes, err := utils.ScreenShotPageTemplate("zhihu-question.html", feed, utils.ScreenShotPageOption{
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
		name: "public-treehole",
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
	mappers[regexp.MustCompile(`https://(?:bbs\.nga\.cn|ngabbs\.com)/read\.php\?tid=(\w+)`)] = generator{
		name: "public-nga-post",
		gen: func(matched []string) ([]byte, error) {
			fp := gofeed.NewParser()
			feed, err := fp.ParseURL(fmt.Sprintf("http://localhost:1200/nga/post/%s", matched[1]))
			if err != nil {
				return nil, err
			}
			bytes, err := utils.ScreenShotPageTemplate("nga.html", feed, utils.ScreenShotPageOption{
				Width:    900,
				Height:   0,
				DPI:      0,
				Before:   nil,
				PwOption: utils.DefaultPageOptions,
				Sleep:    0,
			})
			return bytes, err
		},
	}

}
func init() {
	e := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		Brief:             "生成预览",
		PrivateDataFolder: "previewer",
		Help: `回自动识别链接消息并生成预览图片
注：你需要向Bot妈申请权限, 你可以使用/report功能来向Bot妈申请
目前支持：
X(Twitter): 用户的推文（回复的评论不算）
`,
	})
	initMapper(e)
	e.OnCommand("previewer", zero.SuperUserPermission, func(ctx *zero.Ctx) bool {
		return strings.TrimSpace(ctx.State["args"].(string)) == "reload"
	}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		initMapper(e)
		ctx.Send(fmt.Sprintf("重载预览模板成功,共%d", len(mappers)))
	})
	e.OnMessage(func(ctx *zero.Ctx) bool {
		rawMessage := ctx.Event.RawMessage
		for r, v := range mappers {
			//fmt.Printf("%s_1", v.name)
			if r.MatchString(rawMessage) {
				//fmt.Printf("%s", v.name)
				ctx.State["matched"] = r.FindStringSubmatch(rawMessage)
				ctx.State["name"] = v.name
				ctx.State["generator"] = v.gen
				return true
			}
		}
		return false
	}, func(ctx *zero.Ctx) bool {
		if strings.HasPrefix(ctx.State["name"].(string), "public-") {
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
