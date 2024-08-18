package quote

import (
	"fmt"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func init() {
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "生成一张名人名言图片",
		Help:             `- 回复消息的同时/q  ->渲染一张带有被回复人和话的大照片`,
	})
	client := http.Client{}
	engine.OnRegex(`^\[CQ:reply,id=(-?\d+)\].*/q\s*$`, zero.OnlyGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			//获取消息
			msg := ctx.GetMessage(ctx.State["regex_matched"].([]string)[1])
			name := msg.Sender.Name()
			avatar := fmt.Sprintf("http://q4.qlogo.cn/g?b=qq&nk=%d&s=640", msg.Sender.ID)
			content := msg.Elements.String()
			err, bytes := renderQuoteImage(client, name, avatar, content)
			if err != nil {
				ctx.SendChain(message.Text(err.Error()))
				return
			}
			ctx.SendChain(message.ImageBytes(bytes))
		})
}

func renderQuoteImage(client http.Client, name string, avatar string, message string) (error, []byte) {
	postData := url.Values{}
	postData.Set("userName", name)
	postData.Set("userAvatar", avatar)
	postData.Set("message", message)
	logrus.Infof("rendering %v, %s", name, message)
	response, err := client.Post(
		"http://159.75.127.83:8888/quote?dpi=F1_5X&scale=DEVICE&w=500&fullPage&quality=60",
		"application/x-www-form-urlencoded",
		strings.NewReader(postData.Encode()),
	)
	if err != nil {
		return fmt.Errorf("render image error: %v", err), nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("render image error: %s", response.Status), nil
	}
	var imageBytes []byte
	if imageBytes, err = io.ReadAll(response.Body); err != nil {

		return fmt.Errorf("render image error: %v", err), nil
	}
	return err, imageBytes
}
