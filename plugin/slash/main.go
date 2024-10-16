// Package slash https://github.com/Rongronggg9/SlashBot
package slash

import (
	"github.com/wdvxdr1123/ZeroBot/extension"
	"strconv"
	"strings"

	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var (
	// so noisy and try not to use this.
	engine = control.Register("slash", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: true,
		Brief:            `回应动作`,
		Help:             "slash Plugin, Origin from https://github.com/Rongronggg9/SlashBot\n响应诸如 “/打”、“/敲” “/prpr” 之类消息",
	})
)

func init() {

	/*
		Params:
			/rua [CQ:at,qq=123123] || match1 = /rua | match2 = cq... | match3 = id
			match4 match 5 match 6
	*/
	engine.OnMessage(zero.NewPattern().Text(`^/(.+)$`).At().AsRule()).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		model := extension.PatternModel{}
		_ = ctx.Parse(&model)
		// use matchedinfo
		qidToInt64, _ := strconv.ParseInt(model.Matched[1].At(), 10, 64)
		getUserInfo := ctx.CardOrNickName(qidToInt64)
		getPersentUserinfo := ctx.CardOrNickName(ctx.Event.UserID)
		// split info
		info := model.Matched[0].Text()[1]
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(getPersentUserinfo+" "+info+"了"+getUserInfo))
	})

	engine.OnMessage(zero.NewPattern().Text(`^/(.*)$`).AsRule(), func(ctx *zero.Ctx) bool {
		model := extension.PatternModel{}
		_ = ctx.Parse(&model)
		msg := model.Matched[0].Text()[1]
		if _, ok := control.Lookup(msg); ok {
			return false
		}
		ctx.State["rua"] = msg
		return true
	}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		getPatternInfo := ctx.State["rua"].(string)
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(ctx.CardOrNickName(ctx.Event.UserID)+getPatternInfo+"了自己~"))
	})
	engine.OnMessage(zero.NewPattern().Reply().Text(`^/(.+)$`).AsRule()).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		model := extension.PatternModel{}
		_ = ctx.Parse(&model)
		getPatternUserMessageID := model.Matched[0].Reply()
		getPatternInfo := model.Matched[1].Text()[1]
		getSplit := strings.Split(getPatternInfo, " ")
		rsp := ctx.CallAction("get_msg", zero.Params{
			"message_id": getPatternUserMessageID,
		}).Data.String()
		sender := gjson.Get(rsp, "sender.user_id").Int()
		if len(getSplit) == 2 {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(ctx.CardOrNickName(ctx.Event.UserID)+" "+getSplit[0]+"了 "+ctx.CardOrNickName(sender)+getSplit[1]))
		} else {
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(ctx.CardOrNickName(ctx.Event.UserID)+" "+getPatternInfo+"了 "+ctx.CardOrNickName(sender)))
	})
}
