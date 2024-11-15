// Package fabing 虚拟偶像女团 A-SOUL 成员嘉然相关
package fabing

import (
	"fmt"
	"github.com/FloatTech/ZeroBot-Plugin/utils"
	"github.com/FloatTech/floatbox/binary"
	"github.com/FloatTech/floatbox/web"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"net/url"

	fcext "github.com/FloatTech/floatbox/ctxext"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"

	"github.com/FloatTech/ZeroBot-Plugin/plugin/fabing/data"
)

const (
	lolimiURL = "https://api.lolimi.cn"
	raoURL    = lolimiURL + "/API/rao/api.php"
	yanURL    = lolimiURL + "/API/yan/?url=%v"
	xjjURL    = lolimiURL + "/API/tup/xjj.php"
	qingURL   = lolimiURL + "/API/qing/api.php"
	fabingURL = lolimiURL + "/API/fabing/fb.php?name=%v"
)

var engine = control.AutoRegister(&ctrl.Options[*zero.Ctx]{
	DisableOnDefault: false,
	Brief:            "发病",
	Help: "- {prefix}发病 [可选的人名]  给某个人写个小作文\n" +
		"- {prefix}小作文 [可选的人名]  和上面一样\n" +
		"--------下面是管理员可用指令--------\n" +
		"--------下面是Bot主人可用指令--------\n" +
		"- {prefix}教你一篇小作文[作文]",
	PublicDataFolder: "Diana",
})

func remoteText(name string) (string, error) {
	d, err := web.GetData(fmt.Sprintf(fabingURL, url.QueryEscape(name)))
	if err != nil {
		return "", err
	}
	return gjson.Get(binary.BytesToString(d), "data").String(), nil
}
func init() {
	getdb := fcext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		err := data.LoadText(engine.DataFolder() + "Wtf.db")
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return false
		}
		return true
	})

	// 随机发送一篇上面的小作文
	engine.OnFullMatch("小作文", getdb).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			// 绕过第一行发病
			ctx.SendChain(message.Text(data.RandText()))
		})
	// 逆天
	engine.OnCommandGroup([]string{"发病", "小作文"}, getdb).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			name := ctx.NickName()
			var text string
			if utils.Probability(.7) || data.CountText() == 0 {
				var err error
				text, err = remoteText(name)
				if err != nil {
					log.Errorln(err)
					return
				}
			} else {
				text = data.RandText()
			}
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(text))
		})
	// 增加小作文
	engine.OnCommand(`添加小作文`, zero.AdminPermission, getdb).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			err := data.AddText(ctx.State["regex_matched"].([]string)[1])
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
			} else {
				ctx.SendChain(message.Text("记住啦!"))
			}
		})
}
