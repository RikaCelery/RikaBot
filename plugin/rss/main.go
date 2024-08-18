package rss

import (
	"fmt"
	"github.com/FloatTech/floatbox/process"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/fumiama/cron"
	"github.com/mmcdole/gofeed"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type pushedRss struct {
	Link       string
	Gid        int64
	FeedUrl    string
	Published  string
	PushedDate string
}
type rssInfo struct {
	Id         int
	Gid        int
	Feed       string //rss feed
	LastUpdate string
}

func init() {
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "RSS订阅",
		Help: `- rss a <RSS源> 添加RSS订阅
- rss rm <编号或者RSS链接> 移除RSS订阅
- rss ls <编号或者RSS链接> 列出本群所有RSS订阅
- rss run 强制运行rss更新 [仅超级用户]
- rss interval (.*) rss更新时机, cron表达式 [仅超级用户]
`,

		PrivateDataFolder: "rss",
	})
	db := &sql.Sqlite{}
	db.DBPath = engine.DataFolder() + "rss.db"
	err := db.Open(time.Hour)
	if err != nil {
		logrus.Fatal(err)
	} else {
		db.Create("group_rss", &rssInfo{})
		db.DB.Exec(`create table if not exists 'group_rss_pushed'
(
    Link       TEXT    not null,
    Gid        integer not null,
    FeedUrl    TEXT    not null,
    Published  TEXT default '',
    PushedDate date  not null,
    constraint group_rss_pushed_pk
        primary key (FeedUrl, Gid, Link)
);`)

	}
	client := http.Client{}
	fp := gofeed.NewParser()
	var lock = sync.Mutex{}
	var cronId cron.EntryID
	cmd := func() {
		if !lock.TryLock() {
			return
		}
		defer lock.Unlock()
		infos, err := sql.FindAll[rssInfo](db, "group_rss", "")
		if err != nil {
			logrus.Errorf("[rss update cron] FindAll failed, %v", err)
			return
		}
		for _, res := range infos {
			_ = func() error {
				logrus.Infof("[rss update cron] id %d, group %d, feed %s\n", res.Id, res.Gid, res.Feed)
				feed, err := fp.ParseURL(res.Feed)
				if err != nil {
					logrus.Errorf("[rss update cron] update failed,id %d, group %d,  feed %s, err %v", res.Id, res.Gid, res.Feed, err)
					return nil
				}
				slices.Reverse(feed.Items)
				for _, item := range feed.Items {
					if isRssPushed(db, res.Feed, item, int64(res.Gid)) {
						continue
					}
					logrus.Infof("updated %v %v", item.Title, item.Link)
					parse, err := url.Parse(item.Link)
					if err != nil {
						logrus.Errorf("parse error,id %d, group %d,  feed %s, err %v\n", res.Id, res.Gid, res.Feed, err)
						continue
					}
					res.LastUpdate = item.Published
					err, imageBytes := renderRssImage(item, client)
					if err != nil {
						logrus.Errorln(err)
						return nil
					}

					logrus.Infoln("render image success")
					zero.RangeBot(func(id int64, ctx *zero.Ctx) bool {
						groups := ctx.GetGroupList().Array()
						for _, group := range groups {
							if group.Get("group_id").Int() == int64(res.Gid) {
								var safeLink = fmt.Sprintf("%s%s", parse.Host, parse.Path)
								mid := ctx.SendGroupMessage(int64(res.Gid), (message.Message)(
									[]message.MessageSegment{
										message.ImageBytes(imageBytes), message.Text(fmt.Sprintf("#%s\n%s", feed.Title, safeLink)),
									}))
								if mid <= 0 {
									return false
								}
								err := setRssPushed(db, item, res)
								if err != nil {
									logrus.Errorf("insert group_rss_pushed failed: %v", err)
									return false
								}
								err = insertOrUpdateRssInfo(db, res)
								if err != nil {
									logrus.Errorf("insert group_rss failed: %v", err)
									return false
								}
								return false
							}
						}
						return true
					})
				}
				return nil
			}()
		}
		logrus.Infoln("rss cron task done")
	}
	go func() {
		process.GlobalInitMutex.Lock()
		process.SleepAbout1sTo2s()
		addFunc, err := process.CronTab.AddFunc("@every 10m", cmd)
		if err != nil {
			return
		}
		cronId = addFunc
		process.GlobalInitMutex.Unlock()
	}()
	engine.OnFullMatch("rss run", zero.AdminPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		cmd()
	})
	engine.OnRegex("rss interval (.+)", zero.SuperUserPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		addFunc, err := process.CronTab.AddFunc(ctx.State["regex_matched"].([]string)[1], cmd)
		if err != nil {
			ctx.Send(fmt.Sprintf("[ERROR]: %v", err))
			return
		}
		process.CronTab.Remove(cronId)
		cronId = addFunc
	})
	engine.OnPrefix("rss a", zero.OnlyGroup, zero.AdminPermission).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			link := strings.Replace(ctx.Event.Message.String(), "rss a", "", 1)
			link = strings.Trim(link, " ")
			_, err := url.Parse(link)
			if err != nil {
				ctx.Send("链接似乎不是一个合法的URL～")
				return
			}
			res, err := findRss(ctx, db, link)
			if err == nil {
				ctx.Send(fmt.Sprintf("已存在该rss源,ID:%d", res.Id))
				//return
			} else {
				count, err := db.Count("group_rss")
				if err == nil && count != 0 {
					logrus.Warnf("rss a %v", err)
					err := db.Query("select * from group_rss order by id", res)
					res.Id += 1
					if err != nil {
						logrus.Warnf("rss query error %v", err)
						return
					}
				}
				res.Feed = link
				res.Gid = int(ctx.Event.GroupID)
				res.LastUpdate = time.UnixMicro(1000).Format(time.RFC1123Z)
			}
			// pre check
			logrus.Infof("loading rss %s\n", link)
			feed, err := fp.ParseURL(link)
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("RSS源解析成功，开始抓取数据..."))
			if err != nil {
				logrus.Errorln(err)
				ctx.Send(fmt.Sprintf("[ERROR]: 无法解析RSS源 %v", err))
				return
			}
			slices.Reverse(feed.Items)
			err = insertOrUpdateRssInfo(db, res)
			if err != nil {
				fmt.Printf("insert error: %v\n", err)
				return
			}

			for _, item := range feed.Items {
				if isRssPushed(db, link, item, ctx.Event.GroupID) {
					logrus.Infof("[rss exist] %v %v", item.Title, item.Link)
					continue
				}

				fmt.Printf("updated %v, %s\n", item.Title, item.Link)
				err, imageBytes := renderRssImage(item, client)
				if err != nil {
					logrus.Errorln(err)
					return
				}
				logrus.Infoln("render image success")
				parse, err2 := url.Parse(item.Link)
				var safeLink = ""
				if err2 != nil {
					logrus.Errorf("parse error,id %d, group %d,  feed %s, err %v\n", res.Id, res.Gid, res.Feed, err2)
				} else {
					safeLink = fmt.Sprintf("%s%s", parse.Host, parse.Path)
				}
				ctx.SendChain(message.ImageBytes(imageBytes), message.Text(fmt.Sprintf("#%s\n%s", feed.Title, safeLink)))

				res.LastUpdate = item.Published
				err = insertOrUpdateRssInfo(db, res)
				if err != nil {
					fmt.Printf("insert error: %v\n", err)
					return
				}
				setRssPushed(db, item, res)
			}

		})
	engine.OnFullMatchGroup([]string{"rss ls", "rss list"}, zero.OnlyGroup, zero.AdminPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		var msg = ""
		var res = &rssInfo{}
		db.FindFor("group_rss", res, fmt.Sprintf("where Gid = %d", ctx.Event.GroupID), func() error {
			msg += fmt.Sprintf("ID: %d, Url: %s, 最近更新时间: %s\n", res.Id, res.Feed, res.LastUpdate)
			return nil
		})
		if msg == "" {
			msg = "该群组未订阅任何rss源"
		}
		ctx.SendChain(message.Text(msg))
	})
	engine.OnPrefix("rss rm", zero.OnlyGroup, zero.AdminPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {

		idStr := strings.Replace(ctx.Event.Message.String(), "rss rm", "", 1)
		idStr = strings.Trim(idStr, " ")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			ctx.SendChain(message.Text("ID似乎不是一个数字？", err))
			return
		}
		if err := delRss(db, ctx.Event.GroupID, id); err != nil {
			ctx.SendChain(message.Text("ERROR:", err))
			return
		} else {
			ctx.SendChain(message.Text("删除成功"))
		}
	})

}

func findRss(ctx *zero.Ctx, db *sql.Sqlite, link string) (*rssInfo, error) {
	var res = &rssInfo{}
	return res, db.Find("group_rss", res, fmt.Sprintf("where Feed like '%s' and Gid = %d", link, ctx.Event.GroupID))
}

func insertOrUpdateRssInfo(db *sql.Sqlite, res *rssInfo) error {
	return db.Insert("group_rss", res)
}

func setRssPushed(db *sql.Sqlite, item *gofeed.Item, res *rssInfo) error {
	err := db.Insert("group_rss_pushed", &pushedRss{
		Link:      item.Link,
		Gid:       int64(res.Gid),
		FeedUrl:   res.Feed,
		Published: item.Published,
	})
	return err
}

func renderRssImage(item *gofeed.Item, client http.Client) (error, []byte) {
	postData := url.Values{}
	parsed, _ := url.Parse(item.Link)
	postData.Set("title", item.Title)
	postData.Set("favicon", fmt.Sprintf("https://icons.feedercdn.com/%s", parsed.Host))
	postData.Set("content", item.Description)
	if item.PublishedParsed != nil {
		postData.Set("date", item.PublishedParsed.Format("2006-01-02 15:04:05"))
	} else {
		postData.Set("date", item.Published)
	}
	var author string
	if len(item.Authors) > 0 {
		for _, person := range item.Authors {
			author += person.Name + " "
		}
	} else if item.Author != nil {
		author = item.Author.Name
	}
	postData.Set("author", author)
	logrus.Infof("rendering %v, %s", item.Title, item.Link)
	response, err := client.Post(
		"http://159.75.127.83:8888/rss?dpi=F1_5X&scale=DEVICE&w=500&fullPage&quality=60",
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
func isRssPushed(db *sql.Sqlite, feedUrl string, item *gofeed.Item, gid int64) bool {
	return db.CanFind("group_rss_pushed", fmt.Sprintf("where Link='%s' and gid=%d and FeedUrl='%s'", item.Link, gid, feedUrl))
}
func delRss(db *sql.Sqlite, gid int64, rssId int) error {
	var res = &rssInfo{}
	err := db.Find("group_rss", res, fmt.Sprintf("where Id = %d and Gid = %d", rssId, gid))
	if err != nil {
		return err
	}
	logrus.Infof("rss found %v", res)
	err = db.Del("group_rss", fmt.Sprintf("where Id = %d and Gid = %d", rssId, gid))
	if err != nil {
		return err

	}

	return nil
}
