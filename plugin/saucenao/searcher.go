// Package saucenao P站ID/saucenao/ascii2d搜图
package saucenao

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/FloatTech/AnimeAPI/pixiv"

	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/AnimeAPI/ascii2d"
	"github.com/jozsefsallai/gophersauce"

	"github.com/FloatTech/floatbox/binary"
	"github.com/FloatTech/floatbox/file"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
)

const (
	enableHex = 0x10
	unableHex = 0x7fffffff_fffffffd
)

var (
	saucenaocli *gophersauce.Client
)

// MustProvidePicture 消息不存在图片阻塞120秒至有图片，超时返回 false
func MustProvidePicture(ctx *zero.Ctx) bool {
	if ctx.Event.Message[0].Type == "reply" {
		msg := ctx.GetMessage(ctx.Event.Message[0].Data["id"])
		var urls = []string{}
		for _, elem := range msg.Elements {
			if elem.Type == "image" {
				if elem.Data["url"] != "" {
					urls = append(urls, elem.Data["url"])
				}
			}
		}
		if len(urls) > 0 {
			ctx.State["image_url"] = urls
			return true
		}

	}
	if zero.HasPicture(ctx) {
		return true
	}
	// 没有图片就索取
	ctx.SendChain(message.Text("请发送一张图片"))
	next := zero.NewFutureEvent("message", 999, true, ctx.CheckSession(), zero.HasPicture).Next()
	select {
	case <-time.After(time.Second * 120):
		return false
	case newCtx := <-next:
		ctx.State["image_url"] = newCtx.State["image_url"]
		ctx.Event.MessageID = newCtx.Event.MessageID
		return true
	}
}
func init() { // 插件主体
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "以图搜图",
		Help: `- (以图搜图 | 搜索图片 | 以图识图 | source | src? | src？) + [图片]
- 回复一张图也可以 (别急我还没写)
- 设置saucenao api key <你的key> [仅超级用户]
- ^(开启|打开|启用|关闭|关掉|禁用)搜图显示图片$ [仅超级用户]
`,
		// "- 搜图[P站图片ID]"

		PrivateDataFolder: "saucenao",
	})
	apikeyfile := engine.DataFolder() + "apikey.txt"
	if file.IsExist(apikeyfile) {
		key, err := os.ReadFile(apikeyfile)
		if err != nil {
			panic(err)
		}
		saucenaocli, err = gophersauce.NewClient(&gophersauce.Settings{
			MaxResults: 1,
			APIKey:     binary.BytesToString(key),
		})
		if err != nil {
			panic(err)
		}
	}
	// 根据 PID 搜图
	engine.OnRegex(`^pixiv\s*(\d+)$`).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			id, _ := strconv.ParseInt(ctx.State["regex_matched"].([]string)[1], 10, 64)
			ctx.SendChain(message.Text("少女祈祷中......"))
			// 获取P站插图信息
			illust, err := pixiv.Works(id)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			if illust.Pid > 0 {
				name := strconv.FormatInt(illust.Pid, 10)
				var imgs message.Message
				for i := range illust.ImageUrls {
					f := file.BOTPATH + "/" + illust.Path(i)
					n := name + "_p" + strconv.Itoa(i)
					if file.IsNotExist(f) {
						logrus.Debugln("[saucenao]开始下载", n)
						logrus.Debugln("[saucenao]urls:", illust.ImageUrls)
						err1 := illust.DownloadToCache(i)
						if err1 != nil {
							logrus.Debugln("[saucenao]下载err:", err1)
						}
					}
					imgs = append(imgs, message.Image("file:///"+f))
				}
				txt := message.Text(
					"标题: ", illust.Title, "\n",
					"插画ID: ", illust.Pid, "\n",
					"画师: ", illust.UserName, "\n",
					"画师ID: ", illust.UserID, "\n",
					"直链: ", "https://pixivel.moe/detail?id=", illust.Pid,
				)
				if imgs != nil {
					ctx.Send(message.Message{ctxext.FakeSenderForwardNode(ctx, txt),
						ctxext.FakeSenderForwardNode(ctx, imgs...)})
				} else {
					// 图片下载失败，仅发送文字结果
					ctx.SendChain(txt)
				}
			} else {
				ctx.SendChain(message.Text("图片不存在!"))
			}
		})
	// 以图搜图
	engine.OnMessage(zero.NewPattern(nil).Reply().SetOptional().Text(`^(以图搜图|搜索图片|搜图|识图|以图识图|source|src[?？])`).AsRule(), MustProvidePicture).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			// 开始搜索图片
			pics, ok := ctx.State["image_url"].([]string)
			showPic := false
			if !ok {
				// ctx.SendChain(message.Text("ERROR: 未获取到图片链接"))
				return
			}
			c, ok := ctx.State["manager"].(*ctrl.Control[*zero.Ctx])
			if ok && c.GetData(ctx.Event.GroupID)&enableHex == enableHex {
				showPic = true
			}
			ctx.SendChain(message.Text("少女祈祷中..."))
			for _, pic := range pics {
				if saucenaocli != nil {
					resp, err := saucenaocli.FromURL(pic)
					if err == nil && resp.Count() > 0 {
						result := resp.First()
						s, err := strconv.ParseFloat(result.Header.Similarity, 64)
						if err == nil {
							rr := reflect.ValueOf(&result.Data).Elem()
							b := binary.NewWriterF(func(w *binary.Writer) {
								r := rr.Type()
								for i := 0; i < r.NumField(); i++ {
									if !rr.Field(i).IsZero() {
										w.WriteString("\n")
										w.WriteString(r.Field(i).Name)
										w.WriteString(": ")
										w.WriteString(fmt.Sprint(rr.Field(i).Interface()))
									}
								}
							})
							resp, err := http.Head(result.Header.Thumbnail)
							msg := make(message.Message, 0, 3)
							if s > 80.0 {
								msg = append(msg, message.Text("我有把握是这个!"))
							} else {
								msg = append(msg, message.Text("也许是这个?"))
							}
							if showPic {
								if err == nil {
									_ = resp.Body.Close()
									if resp.StatusCode == http.StatusOK {
										msg = append(msg, message.Image(result.Header.Thumbnail))
									} else {
										msg = append(msg, message.Image(pic))
									}
								} else {
									msg = append(msg, message.Image(pic))
								}
							}
							msg = append(msg, message.Text("\n图源: ", result.Header.IndexName, binary.BytesToString(b)))
							ctx.Send(message.Message{ctxext.FakeSenderForwardNode(ctx, msg...)})
							if s > 80.0 {
								continue
							}
						}
					}
				} else {
					ctx.SendChain(message.Text("请私聊发送 设置 saucenao api key [apikey] 以启用 saucenao 搜图 (方括号不需要输入), key 请前往 https://saucenao.com/user.php?page=search-api 获取"))
				}
				// ascii2d 搜索
				result, err := ascii2d.ASCII2d(pic)
				if err != nil {
					ctx.SendChain(message.Text("ERROR: ", err))
					continue
				}
				msg := message.Message{ctxext.FakeSenderForwardNode(ctx, message.Text("ascii2d搜图结果"))}
				for i := 0; i < len(result) && i < 5; i++ {
					var resultMsgs message.Message
					if showPic {
						resultMsgs = append(resultMsgs, message.Image(result[i].Thumb))
					}
					resultMsgs = append(resultMsgs, message.Text(fmt.Sprintf(
						"标题: %s\n图源: %s\n画师: %s\n画师链接: %s\n图片链接: %s",
						result[i].Name,
						result[i].Type,
						result[i].AuthNm,
						result[i].Author,
						result[i].Link,
					)))
					msg = append(msg, ctxext.FakeSenderForwardNode(ctx, resultMsgs...))
				}
				if id := ctx.Send(msg).ID(); id == 0 {
					ctx.SendChain(message.Text("ERROR: 可能被风控了"))
				}
			}
		})
	engine.OnRegex(`^设置\s?saucenao\s?api\s?key\s?([0-9a-f]{40})$`, zero.SuperUserPermission, zero.OnlyPrivate).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			var err error
			saucenaocli, err = gophersauce.NewClient(&gophersauce.Settings{
				MaxResults: 1,
				APIKey:     ctx.State["regex_matched"].([]string)[1],
			})
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			err = os.WriteFile(apikeyfile, binary.StringToBytes(saucenaocli.APIKey), 0644)
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.SendChain(message.Text("成功!"))
		})
	engine.OnRegex(`^(开启|打开|启用|关闭|关掉|禁用)搜图显示图片$`, zero.AdminPermission).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			gid := ctx.Event.GroupID
			if gid <= 0 {
				// 个人用户设为负数
				gid = -ctx.Event.UserID
			}
			option := ctx.State["regex_matched"].([]string)[1]
			c, ok := ctx.State["manager"].(*ctrl.Control[*zero.Ctx])
			if !ok {
				ctx.SendChain(message.Text("找不到服务!"))
				return
			}
			data := c.GetData(ctx.Event.GroupID)
			switch option {
			case "开启", "打开", "启用":
				data |= enableHex
			case "关闭", "关掉", "禁用":
				data &= unableHex
			default:
				return
			}
			err := c.SetData(gid, data)
			if err != nil {
				ctx.SendChain(message.Text("出错啦: ", err))
				return
			}
			ctx.SendChain(message.Text("已", option, "搜图显示图片"))
		})
}
