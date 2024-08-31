package huntercode

import (
	"fmt"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"strings"
	"time"
)

var (
	engine = control.AutoRegister(&ctrl.Options[*zero.Ctx]{DisableOnDefault: false, Brief: "怪猎集会码", Help: `
- {prefix}世界 <集会码> [公开]
	怪猎世界集会码
	系统自动储存id 时间，集会码 
	回复：世界集会码已储存
- {prefix}崛起 <集会码> [公开]
	怪猎崛起集会码
	系统自动储存id 时间，集会码
	回复：崛起集会码已储存
- {prefix}清除集会码 (群主or管理员)
	所记录的集会码(来自本群的)被删除
- {prefix}世界集会码
	列出可用世界集会码
- {prefix}崛起集会码
	列出可用崛起集会码
`})
	db = &sql.Sqlite{DBPath: engine.DataFolder() + "huntercodes.db"}
)

type code struct {
	Id       int
	Value    string
	Time     int64
	IsPublic bool
	GroupId  int64
	Type     string
}

func (c code) PublicStr() string {
	if c.IsPublic {
		return "公开"
	}
	return "私密"
}

func (c code) HumanReadableTime() string {
	t := time.Unix(c.Time, 0)
	diff := time.Now().Sub(t)
	if diff.Hours() < 1 {
		return fmt.Sprintf("%.1f分钟前", diff.Minutes())
	}
	return t.Format("15:04:05")
}

// insert 忽略ID
func insert(c *code) error {
	_, err := db.DB.Exec(`replace into codes(Value,Time,IsPublic,GroupId,Type) values(?,?,?,?,?)`, c.Value, c.Time, c.IsPublic, c.GroupId, c.Type)
	return err
}
func remove(gid int64, codeType string) error {
	err := db.Del("codes", "WHERE GroupId=? and Type=? ", gid, codeType)
	return err
}
func canFind(gid int64, codeType string, isPublic bool) bool {
	return db.CanFind("codes", "WHERE GroupId=? and Type=? and IsPublic=?", gid, codeType, isPublic)
}
func find(gid int64, codeType string, isPublic bool) (c *code, err error) {
	err = db.Find("codes", c, "WHERE GroupId=? and Type=? and IsPublic=?", gid, codeType, isPublic)
	return c, err
}

func findFor(gid int64, codeType string) []*code {
	ret := []*code{}
	all, err := sql.FindAll[code](db, "codes", "where GroupId=? and Type=?", gid, codeType)
	if err == nil {
		ret = append(ret, all...)
	}
	return ret
}

func init() {
	err := db.Open(time.Hour)
	if err != nil {
		panic(err)
	}
	db.DB.Exec(`
create table if not exists codes(
    Id integer not null primary key autoincrement,
    Value text not null,
    Time integer not null,
    IsPublic integer not null,
    GroupId integer not null,
    Type text not null
);
create unique index if not exists codes_value_groupid_uindex ON codes
    (Value,GroupId,Type)
`)
	engine.OnCommand("清除集会码", zero.OnlyGroup, zero.AdminPermission, zero.CheckArgs(func(ctx *zero.Ctx, args []string) bool {
		if len(args) == 0 {
			// 确认删除所有
			ctx.SendChain(message.Text("你确定要删除所有的集会码吗？【输入Y确认】"))
			next := zero.NewFutureEvent("message", 999, true, ctx.CheckSession()).Next()
			select {
			case <-time.After(time.Second * 120):
				return false
			case newCtx := <-next:
				if strings.ToUpper(newCtx.ExtractPlainText()) != "Y" {
					ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("已取消"))
					return false
				}
				ctx.State["del_all"] = true
				return true
			}
		}
		ctx.State["del_all"] = false
		return true
	})).Handle(func(ctx *zero.Ctx) {
		if ctx.State["del_all"].(bool) {
			err := remove(ctx.Event.GroupID, "世界")
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			err = remove(ctx.Event.GroupID, "崛起")
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			ctx.SendChain(message.Text("已清除本群的集会码"))
		} else {
			err := remove(ctx.Event.GroupID, ctx.State["args"].(string))
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			err = remove(ctx.Event.GroupID, ctx.State["args"].(string))
			if err != nil {
				ctx.SendChain(message.Text("ERROR:", err))
				return
			}
			ctx.SendChain(message.Text(fmt.Sprintf("已清除本群的%s集会码", ctx.State["args"].(string))))
		}
	})
	engine.OnCommandGroup([]string{"世界集会码", "崛起集会码"}, zero.OnlyGroup).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		switch ctx.State["command"].(string) {
		case "世界集会码":
			codes := findFor(ctx.Event.GroupID, "世界")
			if len(codes) > 0 {
				buf := strings.Builder{}
				for _, c := range codes {
					buf.WriteString(fmt.Sprintf("%d: %s %s %s\n", c.Id, c.Value, c.HumanReadableTime(), c.PublicStr()))
				}
				ctx.SendChain(message.Text("世界集会码：\n", strings.TrimSpace(buf.String())))
			} else {
				ctx.SendChain(message.Text("世界集会码：无"))
			}
		case "崛起集会码":
			codes := findFor(ctx.Event.GroupID, "崛起")
			if len(codes) > 0 {
				buf := strings.Builder{}
				for _, c := range codes {
					buf.WriteString(fmt.Sprintf("%d: %s %s %s\n", c.Id, c.Value, c.HumanReadableTime(), c.PublicStr()))
				}
				ctx.SendChain(message.Text("崛起集会码：\n", strings.TrimSpace(buf.String())))
			} else {
				ctx.SendChain(message.Text("崛起集会码：无"))
			}
		}
	})
	engine.OnCommandGroup([]string{"世界", "崛起"}, zero.OnlyGroup, zero.CheckArgs(func(ctx *zero.Ctx, args []string) bool {
		//TODO check format
		if len(args) == 1 {
			args = append(args, "公开")
		}
		ctx.State["args"] = args
		return true
	})).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		args := ctx.State["args"].([]string)
		if len(args) < 2 {
			ctx.SendChain(message.Text("参数错误"))
			return
		}
		err := insert(&code{
			Id:       -1,
			Value:    args[0],
			Time:     time.Now().Unix(),
			IsPublic: args[1] == "公开",
			GroupId:  ctx.Event.GroupID,
			Type:     ctx.State["command"].(string),
		})
		if err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		}
		ctx.SendChain(message.Text(fmt.Sprintf("%s集会码已储存", ctx.State["command"])))
	})
}
