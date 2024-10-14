// Package lolicon 基于 https://api.lolicon.app 随机图片
package lolicon

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/floatbox/math"
	"github.com/FloatTech/floatbox/process"
	"github.com/FloatTech/floatbox/web"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
)

const (
	api      = "https://api.lolicon.app/setu/v2"
	capacity = 10
)

var (
	queue     = make(chan string, capacity)
	customapi = ""
)

func init() {
	en := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "随机图片",
		Help: "- 随机图片\n" +
			"- 随机图片 萝莉|少女\n" +
			"- 设置随机图片地址[http...]",
	}).ApplySingle(ctxext.DefaultSingle)
	en.OnPrefix("随机图片").Limit(ctxext.LimitByUser).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			if imgtype := strings.TrimSpace(ctx.State["args"].(string)); imgtype != "" {
				buf := strings.Builder{}
				buf.WriteString(api)
				split := strings.Split(imgtype, "&")
				for i := range split {
					if i == 0 {
						buf.WriteByte('?')
					} else {
						buf.WriteByte('&')
					}
					buf.WriteString(fmt.Sprintf("tag=%s", url.QueryEscape(split[i])))
				}
				log.Debugf("api: %v", buf.String())
				imageurl, err := getimgurl(buf.String())
				if err != nil {
					ctx.SendChain(message.Text("ERROR: ", err))
					return
				}
				data, err := http.Get(imageurl)
				if err != nil {
					ctx.SendChain(message.Text("ERROR: ", err))
					return
				}
				var b = bytes.Buffer{}
				defer data.Body.Close()
				_, err = io.Copy(&b, data.Body)
				if err != nil {
					ctx.SendChain(message.Text("ERROR: ", err))
					return
				}
				if id := ctx.Send(message.Message{ctxext.FakeSenderForwardNode(ctx, message.ImageBytes(b.Bytes()))}).ID(); id == 0 {
					ctx.SendChain(message.Text("ERROR: 可能被风控了"))
				}
				return
			}
			go func() {
				for i := 0; i < math.Min(cap(queue)-len(queue), 2); i++ {
					if customapi != "" {
						data, err := web.GetData(customapi)
						if err != nil {
							ctx.SendChain(message.Text("ERROR: ", err))
							continue
						}
						queue <- "base64://" + base64.StdEncoding.EncodeToString(data)
						continue
					}
					imageurl, err := getimgurl(api)
					if err != nil {
						ctx.SendChain(message.Text("ERROR: ", err))
						continue
					}
					queue <- imageurl
				}
			}()
			select {
			case <-time.After(time.Minute):
				ctx.SendChain(message.Text("ERROR: 等待填充，请稍后再试......"))
			case img := <-queue:
				if id := ctx.Send(message.Message{ctxext.FakeSenderForwardNode(ctx, message.Image(img))}).ID(); id == 0 {
					process.SleepAbout1sTo2s()
					if id := ctx.Send(message.Message{ctxext.FakeSenderForwardNode(ctx, message.Image(img).Add("cache", "0"))}).ID(); id == 0 {
						ctx.SendChain(message.Text("ERROR: 可能被风控或下载图片用时过长，请耐心等待"))
					}
				}
			}
		})
	en.OnPrefix("设置随机图片地址", zero.SuperUserPermission).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			u := strings.TrimSpace(ctx.State["args"].(string))
			ctx.SendChain(message.Text("成功设置随机图片地址为", u))
			customapi = u
		})
}

func getimgurl(url string) (string, error) {
	data, err := web.GetData(url)
	if err != nil {
		return "", err
	}
	json := gjson.ParseBytes(data)
	if e := json.Get("error").Str; e != "" {
		return "", errors.New(e)
	}
	var imageurl string
	if imageurl = json.Get("data.0.urls.original").Str; imageurl == "" {
		return "", errors.New("未找到相关内容, 换个tag试试吧")
	}
	return imageurl, nil
}
