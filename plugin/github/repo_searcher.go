// Package github GitHub 仓库搜索
package github

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
	log "github.com/sirupsen/logrus"
	"github.com/wdvxdr1123/ZeroBot/extension"
	"github.com/wdvxdr1123/ZeroBot/extension/shell"

	"github.com/FloatTech/ZeroBot-Plugin/utils"

	"github.com/FloatTech/floatbox/web"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/fumiama/terasu/http2"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/tidwall/gjson"
)

var hiddenCSS = `react-app{
    min-height: unset!important;
}
.gh-header-sticky,
.gh-header-shadow,
.gh-header-show .gh-header-actions,
#issues-index-tip,
.hlWueK{
    position: static !important;
    display: none !important;
}
#repos-sticky-header{
    position: relative !important;
}
turbo-frame {
    padding-top: 10px;
    padding-bottom: 20px;
}

.discussion-timeline-actions {
    display: none;
}
`

func init() { // 插件主体
	e := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "GitHub相关",
		Help: "- {prefix}github [xxx]\n" +
			"- {prefix}github -p [xxx]",
	})
	e.OnRegex(`(github\.com/([^/ \n]+)/([^/ \n]+)(?:(/pulls?|/issues|/discussions|/actions(?:/runs)?|/blob)/?(\d+)?)?[^#\s]*(#L\d+-L\d+)?)`).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			log.Debugf("[github] regex matched: %v", ctx.State["regex_matched"].([]string))
			model := &extension.RegexModel{}
			_ = ctx.Parse(model)
			if model.Matched[2] == "topics" {
				return
			}
			switch expression := model.Matched[4]; expression {
			case "":
				data, err := web.GetData("https://opengraph.githubassets.com/0/" + model.Matched[2] + "/" + model.Matched[3])
				if err != nil {
					log.Errorln("[ERROR]:", err)
				}
				ctx.Send(message.ImageBytes(data))
				return
			case "/blob":
				bytes, err := utils.ScreenShotElementURL(
					model.Matched[1],
					"turbo-frame",
					utils.ScreenShotElementOption{Width: 1000,
						DPI:   1,
						Sleep: time.Millisecond * 1000, PwOption: playwright.LocatorScreenshotOptions{
							Style: playwright.String(utils.GlobalCSS + "\n" + hiddenCSS),
						}},
				)
				if err != nil {
					log.Errorln(err)
					ctx.Send(fmt.Sprintf("ERROR: %v", err))
					return
				}
				ctx.Send(message.ImageBytes(bytes).Add("cache", 0))
			case "/actions/runs":
				fallthrough
			case "/issues":
				fallthrough
			case "/discussions":
				fallthrough
			case "/pull":
				fallthrough
			case "/actions":
				fallthrough
			case "/pulls":
				fallthrough
			default:
				bytes, err := utils.ScreenShotElementURL(
					model.Matched[1],
					"turbo-frame",
					utils.ScreenShotElementOption{Width: 850,
						DPI:   1,
						Sleep: time.Millisecond * 1000, PwOption: playwright.LocatorScreenshotOptions{
							Style: playwright.String(utils.GlobalCSS + "\n" + hiddenCSS),
						}},
				)
				if err != nil {
					log.Errorln(err)
					return
				}
				ctx.Send(message.ImageBytes(bytes).Add("cache", 0))
			}
		})
	e.OnCommand(`github`).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			// 发送请求
			args := shell.Parse(ctx.State["args"].(string))
			api, _ := url.Parse("https://api.github.com/search/repositories")
			api.RawQuery = url.Values{
				"q": []string{args[len(args)-1]},
			}.Encode()
			body, err := web.RequestDataWithHeaders(&http2.DefaultClient, api.String(), "GET", func(r *http.Request) error {
				r.Header.Set("User-Agent", web.RandUA())
				return nil
			}, nil)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
			}
			// 解析请求
			info := gjson.ParseBytes(body)
			if info.Get("total_count").Int() == 0 {
				ctx.SendChain(message.Text("ERROR: 没有找到这样的仓库"))
				return
			}
			repo := info.Get("items.0")
			// 发送结果
			switch args[0] {
			case "-p ": // 图片模式
				ctx.SendChain(
					message.Image(
						"https://opengraph.githubassets.com/0/"+repo.Get("full_name").Str,
					).Add("cache", 0),
				)
			case "-t ": // 文字模式
				ctx.SendChain(
					message.Text(
						repo.Get("full_name").Str, "\n",
						"Description: ",
						repo.Get("description").Str, "\n",
						"Star/Fork/Issue: ",
						repo.Get("watchers").Int(), "/", repo.Get("forks").Int(), "/", repo.Get("open_issues").Int(), "\n",
						"Language: ",
						notnull(repo.Get("language").Str), "\n",
						"License: ",
						notnull(strings.ToUpper(repo.Get("license.key").Str)), "\n",
						"Last pushed: ",
						repo.Get("pushed_at").Str, "\n",
						"Jump: ",
						repo.Get("html_url").Str, "\n",
					),
				)
			default: // 文字模式
				ctx.SendChain(
					message.Text(
						repo.Get("full_name").Str, "\n",
						"Description: ",
						repo.Get("description").Str, "\n",
						"Star/Fork/Issue: ",
						repo.Get("watchers").Int(), "/", repo.Get("forks").Int(), "/", repo.Get("open_issues").Int(), "\n",
						"Language: ",
						notnull(repo.Get("language").Str), "\n",
						"License: ",
						notnull(strings.ToUpper(repo.Get("license.key").Str)), "\n",
						"Last pushed: ",
						repo.Get("pushed_at").Str, "\n",
						"Jump: ",
						repo.Get("html_url").Str, "\n",
					),
					message.Image(
						"https://opengraph.githubassets.com/0/"+repo.Get("full_name").Str,
					).Add("cache", 0),
				)
			}
		})
}

// notnull 如果传入文本为空，则返回默认值
func notnull(text string) string {
	if text == "" {
		return "None"
	}
	return text
}
