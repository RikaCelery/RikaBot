// Package browser playwright浏览器相关
package browser

import (
	"fmt"
	"github.com/FloatTech/ZeroBot-Plugin/utils"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func init() {
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: true,
		Brief:            "浏览器",
		Help:             "- 截图 <网址>",
	})
	engine.OnCommand("截图").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		img, err := utils.ScreenShotPageURL(ctx.State["args"].(string))
		if err != nil {
			ctx.Send(fmt.Sprintf("ERROR: %v", err))
			return
		}
		ctx.Send(message.ImageBytes(img))
	})
}
