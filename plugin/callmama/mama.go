// Package callmama is a plugin to call mama.
package callmama

import (
	"os"
	"strconv"
	"strings"
	"time"

	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	log "github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

type lastMessage struct {
	ID   int64
	Time int64
}

func init() {
	e := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		Brief:             "妈！",
		Help:              `妈妈妈妈妈妈妈！`,
		PrivateDataFolder: "mama",
	}).ApplySingle(ctxext.NoHintSingle)
	db := sql.Sqlite{DBPath: e.DataFolder() + "/mama.db"}
	err := db.Open(time.Minute)
	if err != nil {
		panic(err)
	}
	err = db.Create("last_speak", &lastMessage{})
	if err != nil {
		panic(err)
	}
	e.OnMessage(func(ctx *zero.Ctx) bool {
		dir, _ := os.ReadDir(e.DataFolder())
		for _, entry := range dir {
			if strings.HasSuffix(entry.Name(), ".mama") {
				id, err := strconv.ParseInt(strings.ReplaceAll(entry.Name(), ".mama", ""), 64, -1)
				if err != nil {
					log.Errorln(err)
					continue
				}
				if ctx.Event.Sender.ID == id {
					goto true
				}
			}
		}
		return false
	true:
		if db.CanFind("last_speak", "where id=?", ctx.Event.UserID) {
			msg := lastMessage{}
			_ = db.Find("last_speak", &msg, "where id=?", ctx.Event.UserID)
			err := db.Insert("last_speak", &lastMessage{
				ID:   ctx.Event.UserID,
				Time: time.Now().Unix(),
			})
			if err != nil {
				log.Errorln(err)
				return false
			}
			return time.Now().Unix()-msg.Time >= 60*10
		} else {
			err := db.Insert("last_speak", &lastMessage{
				ID:   ctx.Event.UserID,
				Time: time.Now().Unix(),
			})
			if err != nil {
				log.Errorln(err)
			}
		}
		return false
	}).Handle(func(ctx *zero.Ctx) {
		ctx.SendChain(message.At(ctx.Event.UserID), message.Text(" 妈！"))
	})
}
