package niuniu

import (
	"time"

	"github.com/RomiChan/syncx"
	"github.com/fumiama/slowdo"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

var slowsenders = syncx.Map[int64, *syncx.Lazy[*slowdo.Job[*zero.Ctx, message.Message]]]{}
var slowsendersfast = syncx.Map[int64, *syncx.Lazy[*slowdo.Job[*zero.Ctx, message.Message]]]{}

func collectSendFast(ctx *zero.Ctx, msgs ...message.Segment) {
	id := ctx.Event.GroupID
	if id == 0 {
		// only support group
		return
	}
	lazy, _ := slowsendersfast.LoadOrStore(id, &syncx.Lazy[*slowdo.Job[*zero.Ctx, message.Message]]{
		Init: func() *slowdo.Job[*zero.Ctx, message.Message] {
			x, err := slowdo.NewJob(time.Second*10, ctx, func(ctx *zero.Ctx, msg []message.Message) {
				m := make(message.Message, len(msg))
				for i, item := range msg {
					m[i] = message.CustomNode(
						zero.BotConfig.NickName[0],
						ctx.Event.SelfID,
						item)
				}
				ctx.SendGroupForwardMessage(id, m)
			})
			if err != nil {
				panic(err)
			}
			return x
		},
	})
	job := lazy.Get()
	job.Add(msgs)
}
func collectsend(ctx *zero.Ctx, msgs ...message.Segment) {
	id := ctx.Event.GroupID
	if id == 0 {
		// only support group
		return
	}
	lazy, _ := slowsenders.LoadOrStore(id, &syncx.Lazy[*slowdo.Job[*zero.Ctx, message.Message]]{
		Init: func() *slowdo.Job[*zero.Ctx, message.Message] {
			x, err := slowdo.NewJob(time.Second*60, ctx, func(ctx *zero.Ctx, msg []message.Message) {
				m := make(message.Message, len(msg))
				for i, item := range msg {
					m[i] = message.CustomNode(
						zero.BotConfig.NickName[0],
						ctx.Event.SelfID,
						item)
				}
				ctx.SendGroupForwardMessage(id, m)
			})
			if err != nil {
				panic(err)
			}
			return x
		},
	})
	job := lazy.Get()
	job.Add(msgs)
}
