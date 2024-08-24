package igem

import (
	"bytes"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/single"
	"github.com/wdvxdr1123/ZeroBot/message"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func init() {
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Help:             "igem\n- igem <link>",
	}).ApplySingle(single.New(
		single.WithKeyFn(func(ctx *zero.Ctx) int64 {
			return ctx.Event.GroupID
		}),
		single.WithPostFn[int64](func(ctx *zero.Ctx) {
			ctx.Send("您有操作正在执行, 请稍后再试!")
		}),
	))
	client := &http.Client{
		Timeout: time.Minute * 5,
	}
	engine.OnCommand("igem").SetBlock(true).Handle(func(ctx *zero.Ctx) {

		link := ctx.State["args"].(string)
		link = strings.TrimSpace(link)
		p, _ := url.Parse("http://localhost:8888/igem")
		q := url.Values{}
		q.Set("url", link)
		p.RawQuery = q.Encode()
		ctx.Send("请稍后...")
		resp, err := client.Get(p.String())
		if err != nil {
			ctx.SendChain(message.Text("ERROR: " + err.Error()))
			return
		}
		if resp.StatusCode != 200 {
			ctx.SendChain(message.Text("ERROR: code:", resp.Status))
			return
		}
		buf := bytes.Buffer{}
		defer resp.Body.Close()
		_, err = io.Copy(&buf, resp.Body)
		if err != nil {
			ctx.SendChain(message.Text("ERROR: " + err.Error()))
			return
		}
		ctx.Send(message.ImageBytes(buf.Bytes()))
	})
}
