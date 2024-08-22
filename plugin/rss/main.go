package rss

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FloatTech/floatbox/process"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/PuerkitoBio/goquery"
	"github.com/alexflint/go-arg"
	"github.com/fumiama/cron"
	"github.com/mattn/go-runewidth"
	"github.com/mmcdole/gofeed"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/shell"
	"github.com/wdvxdr1123/ZeroBot/message"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"regexp"
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
		Help: `- rss a <RSS源> [-i]  添加RSS订阅
- rss t <RSS源>  测试RSS输出样式
- rss format <编号> <样式ID>
	样式ID:
	1: 默认浏览器渲染内容截图,带链接
	2: 带链接和标题
	3: 带链接和标题和图片
	4: 带链接和标题和图片和内容
	5: 自定义
}
}
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
		err = db.Create("group_rss", &rssInfo{})
		if err != nil {
			panic(err)
		}
		_, err = db.DB.Exec(`create table if not exists 'group_rss_pushed'
(
    Link       TEXT    not null,
    Gid        integer not null,
    FeedUrl    TEXT    not null,
    Published  TEXT default '',
    PushedDate date  not null,
    constraint group_rss_pushed_pk
        primary key (FeedUrl, Gid, Link)
);
create table if not exists 'group_rss_format'
(
    Gid        integer not null,
    FeedUrl    TEXT    not null,
    RenderType integer not null,
    template   TEXT    null,
    constraint group_rss_pushed_pk
        primary key (FeedUrl, Gid)
);
`)
		if err != nil {
			panic(err)
		}

	}
	client := &http.Client{}
	fp := gofeed.NewParser()
	var lock = sync.Mutex{}
	var cronId cron.EntryID
	cronTask := func() {
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
					zero.RangeBot(func(id int64, ctx *zero.Ctx) bool {
						groups := ctx.GetGroupList().Array()
						for _, group := range groups {
							if group.Get("group_id").Int() == int64(res.Gid) {

								err, _ := sendRssMessage(db, item, client, feed, ctx, res)
								res.LastUpdate = item.Published
								err = setRssPushed(db, item, res)
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
		addFunc, err := process.CronTab.AddFunc("@every 10m", cronTask)
		if err != nil {
			return
		}
		cronId = addFunc
		process.GlobalInitMutex.Unlock()
	}()
	engine.OnFullMatch("rss run", zero.AdminPermission).
		SetBlock(true).Handle(func(ctx *zero.Ctx) {
		cronTask()
	})

	engine.OnRegex("rss +interval +(.+)", zero.SuperUserPermission).
		SetBlock(true).Handle(func(ctx *zero.Ctx) {
		addFunc, err := process.CronTab.AddFunc(ctx.State["regex_matched"].([]string)[1], cronTask)
		if err != nil {
			ctx.Send(fmt.Sprintf("[ERROR]: %v", err))
			return
		}
		process.CronTab.Remove(cronId)
		cronId = addFunc
	})

	var argRssAdd struct {
		URL          string `arg:"positional"`
		IgnoreBefore bool   `arg:"-i" help:"只推送之后的新文章"`
	}
	argRssAddParser, _ := arg.NewParser(arg.Config{Program: "rss a", IgnoreEnv: true}, &argRssAdd)
	engine.OnRegex("rss +a +(.+)", zero.OnlyGroup, zero.AdminPermission).
		SetBlock(true).Handle(func(ctx *zero.Ctx) {

		err := argRssAddParser.Parse(shell.Parse(ctx.State["regex_matched"].([]string)[1]))
		if err != nil {
			var buf = &strings.Builder{}
			buf.WriteString("用法似乎不对哦\n")
			argRssAddParser.WriteHelp(buf)
			ctx.Send(buf.String())
			return
		}
		link := argRssAdd.URL
		_, err = url.Parse(link)
		if err != nil {
			return
		}
		res, err := findRss(ctx, db, link)
		if err == nil {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("已存在该rss源,ID:%d", res.Id)))
			return
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
		if err != nil {
			logrus.Errorf("failed to load rss: %v", err)
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("[ERROR]: 无法解析RSS源 %v", err)))
			return
		}
		err = insertOrUpdateRssInfo(db, res)
		if err != nil {
			logrus.Errorf("insert error: %v\n", err)
			return
		}
		if argRssAdd.IgnoreBefore {
			for _, item := range feed.Items {
				if isRssPushed(db, res.Feed, item, int64(res.Gid)) {
					continue
				}
				err := setRssPushed(db, item, res)
				if err != nil {
					logrus.Errorf("setRssPushed failed: %v", err)
					return
				}
			}
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("RSS源添加成功，将在下一次更新时推送内容"))

	})

	var argRssTest struct {
		URL    string `arg:"positional"`
		Format int    `arg:"-f" default:"1"`
	}
	argRssTestParser, _ := arg.NewParser(arg.Config{Program: "rss t", IgnoreEnv: true}, &argRssTest)
	engine.OnRegex("rss +t +(.*)", zero.OnlyGroup, zero.AdminPermission).Handle(func(ctx *zero.Ctx) {
		err := argRssTestParser.Parse(shell.Parse(ctx.State["regex_matched"].([]string)[1]))
		if err != nil {
			logrus.Infoln(err.Error())
			var buf = &strings.Builder{}
			buf.WriteString("用法似乎不对哦\n")
			argRssTestParser.WriteHelp(buf)
			ctx.Send(buf.String())
			return
		}
		feed, err := fp.ParseURL(argRssTest.URL)
		if err != nil {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("[ERROR]: 无法解析RSS源 %v", err)))
			return
		}
		var res = rssInfo{
			Feed:       argRssTest.URL,
			Gid:        int(ctx.Event.GroupID),
			LastUpdate: time.UnixMicro(1000).Format(time.RFC1123Z),
		}
		err, _ = sendRssMessageFormat(db, feed.Items[0], client, feed, ctx, &res, argRssTest.Format)
		if err != nil {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("[ERROR]: %v", err)))
			return
		}
	})

	engine.OnRegex("rss +format +(\\d+) +(\\d)", zero.OnlyGroup, zero.AdminPermission).
		SetBlock(true).Handle(func(ctx *zero.Ctx) {
		rssId, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
		rssinfo, err := findRssById(ctx, db, rssId)
		if err != nil {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("未找到该RSS源"))
			return
		}
		msgType, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[2])
		switch msgType {
		case 1:
			err := setRssRenderType(db, 1, ctx.Event.GroupID, rssinfo.Feed, "")
			if err != nil {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("[ERROR]:%v", err)))
				return
			}
		case 2:
			err := setRssRenderType(db, 2, ctx.Event.GroupID, rssinfo.Feed, "")
			if err != nil {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("[ERROR]:%v", err)))
				return
			}
		case 3:
			err := setRssRenderType(db, 3, ctx.Event.GroupID, rssinfo.Feed, "")
			if err != nil {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("[ERROR]:%v", err)))
				return
			}
		case 4:
			err := setRssRenderType(db, 4, ctx.Event.GroupID, rssinfo.Feed, "")
			if err != nil {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("[ERROR]:%v", err)))
				return
			}
		case 5:
			err := setRssRenderType(db, 5, ctx.Event.GroupID, rssinfo.Feed, regexp.MustCompile("rss +format +(\\d+) +(\\d) *\n?").ReplaceAllString(ctx.MessageString(), ""))
			if err != nil {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("[ERROR]:%v", err)))
				return
			}
		default:
			var buf = &strings.Builder{}
			buf.WriteString("用法似乎不对哦~,类型:\n1: 默认，渲染html截图\n2: 推送标题、链接\n3: 推送标题、链接和图片\n4: 推送标题、链接和图片和内容\n5: 自定义消息模板")
			ctx.Send(buf.String())
			return
		}
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("RSS格式设置成功，将在下一次推送时生效"))

	})

	engine.OnFullMatchGroup([]string{"rss ls", "rss list"}, zero.OnlyGroup, zero.AdminPermission).
		SetBlock(true).Handle(func(ctx *zero.Ctx) {
		var msg = ""
		var res = &rssInfo{}
		err := db.FindFor("group_rss", res, fmt.Sprintf("where Gid = %d", ctx.Event.GroupID), func() error {
			msg += fmt.Sprintf("ID: %d, Url: %s, 最近更新时间: %s\n", res.Id, res.Feed, res.LastUpdate)
			return nil
		})
		if err != nil {
			ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
			return
		}
		if msg == "" {
			msg = "该群组未订阅任何rss源"
		}
		ctx.SendChain(message.Text(msg))
	})

	engine.OnPrefix("rss rm", zero.OnlyGroup, zero.AdminPermission).
		SetBlock(true).Handle(func(ctx *zero.Ctx) {

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
func getTemplate(db *sql.Sqlite, gid int64, feedUrl string) (error, string) {
	var t string
	if db.CanQuery(fmt.Sprintf("select template from group_rss_format where Gid = %d and FeedUrl = '%s'", gid, feedUrl)) {
		row := db.DB.QueryRow(fmt.Sprintf("select template from group_rss_format where Gid = %d and FeedUrl = '%s'", gid, feedUrl))
		err := row.Scan(&t)
		if err != nil {
			return err, ""
		}
		return nil, t
	}
	return errors.New("未找到模板"), ""
}
func getRssRenderType(db *sql.Sqlite, gid int64, feedUrl string) int {
	var t = -1
	if db.CanQuery(fmt.Sprintf("select RenderType from group_rss_format where Gid = %d and FeedUrl = '%s';", gid, feedUrl)) {
		row := db.DB.QueryRow(fmt.Sprintf("select RenderType from group_rss_format where Gid = %d and FeedUrl = '%s';", gid, feedUrl))
		err := row.Scan(&t)
		if err != nil {
			return -1
		}
		return t
	} else {
		return 1
	}
}

func setRssRenderType(db *sql.Sqlite, t int, gid int64, feedUrl string, tmpl string) error {
	_, err := db.DB.Exec("delete from group_rss_format where Gid=? and FeedUrl=?", gid, feedUrl)
	if err != nil {
		return err
	}

	if t == 5 {
		if tmpl == "" {
			return errors.New("模板不能为空")
		}
		_, err := db.DB.Exec("insert into group_rss_format(Gid, FeedUrl, RenderType,template) values(?, ?, ?,?);", gid, feedUrl, t, tmpl)
		return err
	}
	_, err = db.DB.Exec("insert into group_rss_format(Gid, FeedUrl, RenderType) values(?, ?, ?);", gid, feedUrl, t)
	return err
}

func sendRssMessageFormat(db *sql.Sqlite, item *gofeed.Item, client *http.Client, feed *gofeed.Feed, ctx *zero.Ctx, res *rssInfo, t int) (error, int64) {
	err, msg := renderRssToMessage(db, t, item, feed, res, client)
	mid := ctx.SendGroupMessage(int64(res.Gid), msg)
	return err, mid
}
func sendRssMessage(db *sql.Sqlite, item *gofeed.Item, client *http.Client, feed *gofeed.Feed, ctx *zero.Ctx, res *rssInfo) (error, int64) {
	var t = getRssRenderType(db, int64(res.Gid), res.Feed)
	err, msg := renderRssToMessage(db, t, item, feed, res, client)
	mid := ctx.SendGroupMessage(int64(res.Gid), msg)
	return err, mid
}

func truncate(title string, max int) string {
	return runewidth.Truncate(title, max, "...")
}
func templateRender(_template string, item *gofeed.Item, feed *gofeed.Feed) (error, string) {
	funcs := template.FuncMap{
		"truncate": truncate,
		"extractImages": func(in *gofeed.Item) {
			reader, err := goquery.NewDocumentFromReader(strings.NewReader(in.Description))
			if err != nil {
				logrus.Errorf("tohtml error: %v", err)
				panic(err)
			}
			var imgs []string
			if in.Image != nil {
				imgs = append(imgs, fmt.Sprintf("[CQ:image,file=%s]", message.EscapeCQCodeText(in.Image.URL)))
			}
			for _, enclosure := range in.Enclosures {
				if !strings.HasPrefix(enclosure.Type, "image") {
					continue
				}
				cqUrl := fmt.Sprintf("[CQ:image,file=%s]", message.EscapeCQCodeText(enclosure.URL))
				if !slices.Contains(imgs, cqUrl) {
					imgs = append(imgs, cqUrl)
				}
			}
			reader.Find("img").Each(func(i int, selection *goquery.Selection) {
				src := selection.AttrOr("src", "")
				cqUrl := fmt.Sprintf("[CQ:image,file=%s]", message.EscapeCQCodeText(src))
				if !slices.Contains(imgs, cqUrl) {
					imgs = append(imgs, cqUrl)
				}
			})
		},
		"replace": func(src string, reg string, repl string) string {
			regex, err := regexp.Compile(reg)
			if err != nil {
				logrus.Errorf("regexp compile error: %v", err)
				panic(err)
			}
			return regex.ReplaceAllString(src, repl)
		},
		"escape": func(in string) template.HTML {
			return template.HTML(in)
		},
		"tohtml": func(in string) *goquery.Document {
			reader, err := goquery.NewDocumentFromReader(strings.NewReader(in))
			if err != nil {
				logrus.Errorf("tohtml error: %v", err)
				panic(err)
			}
			return reader
		},
		"select": func(in string, selector string) *goquery.Selection {
			reader, err := goquery.NewDocumentFromReader(strings.NewReader(in))
			if err != nil {
				logrus.Errorf("select error: %v", err)
				panic(err)
			}
			find := reader.Find(selector)
			//println(in, selector, find)
			return find
		},
		"selContent": func(in *goquery.Selection) template.HTML {
			return template.HTML(strings.Trim(in.Text(), " \n\r"))
		},
		"docContent": func(in *goquery.Document) template.HTML {
			return template.HTML(strings.Trim(in.Text(), " \n\r"))
		},
		"startWith": func(in string, str string) bool {
			return strings.HasPrefix(in, str)
		},
		"endWith": func(in string, str string) bool {
			return strings.HasSuffix(in, str)
		},
		"isnil": func(any interface{}) bool {
			if any == nil {
				return true
			} else {
				return false
			}
		},
		"notnil": func(any interface{}) bool {
			if any != nil {
				return true
			} else {
				return false
			}
		},
	}

	tmpl, err := template.New("rss_text_template").Funcs(funcs).Parse(_template)
	if err != nil {
		return err, ""
	}
	var buf = &strings.Builder{}
	err = tmpl.Execute(buf, struct {
		Item *gofeed.Item
		Feed *gofeed.Feed
	}{
		Item: item,
		Feed: feed,
	})
	if err != nil {
		return err, ""
	}
	return nil, buf.String()
}

// 1: 默认，渲染html截图
// 2: 推送标题、链接
// 3: 推送标题、链接和图片
// 4: 推送标题、链接和图片和内容
// 5: 自定义消息模板
func renderRssToMessage(db *sql.Sqlite, renderType int, item *gofeed.Item, feed *gofeed.Feed, res *rssInfo, client *http.Client) (error, interface{}) {
	defer func() {
		if err := recover(); err != nil {
			marshal, _ := json.MarshalIndent(item, "", "  ")
			fmt.Printf("%v", string(marshal))
			fmt.Printf("%v", res)
			fmt.Printf("%v", renderType)
			logrus.Errorf("renderRssToMessage panic: %v", err)
		}
	}()
	for i := range item.Categories {
		item.Categories[i] = "#" + item.Categories[i]
	}
	switch renderType {
	case 1:
		err, imageBytes := renderRssImage(item, client)
		if err != nil {
			logrus.Errorln(err)
			return err, nil
		}
		logrus.Infoln("render image success")

		return nil, []message.MessageSegment{
			message.ImageBytes(imageBytes), message.Text(strings.Trim(fmt.Sprintf("#%s\n%s\n%s", feed.Title, item.Link, strings.Join(item.Categories, ", ")), " \n\r")),
		}
	case 2:
		return nil, []message.MessageSegment{message.Text("#"+feed.Title, "\n#", truncate(item.Title, 80), "\n", strings.Join(item.Categories, ", "), "\n", item.Link)}
	case 3:
		var msgs = []message.MessageSegment{message.Text("#"+feed.Title, "\n#", truncate(item.Title, 80), "\n", strings.Join(item.Categories, ", "), "\n", item.Link)}
		var links = []string{}
		if item.Image != nil {
			msgs = append(msgs, message.Image(item.Image.URL))
			links = append(links, item.Image.URL)
		}
		for _, enclosure := range item.Enclosures {
			if strings.HasPrefix(enclosure.Type, "image/") && !slices.Contains(links, enclosure.URL) {
				msgs = append(msgs, message.Image(enclosure.URL))
				links = append(links, enclosure.URL)
			}
		}
		return nil, msgs

	case 4:
		desc := ""
		reader, err := goquery.NewDocumentFromReader(strings.NewReader(item.Description))
		if err != nil {
			desc = strings.Trim(item.Description, " \n\r")
		} else {
			desc = strings.Trim(reader.Text(), " \n\r")
		}
		var msgs = []message.MessageSegment{message.Text("#"+feed.Title, "\n#", item.Title, "\n", strings.Join(item.Categories, ", "), "\n", item.Link, "\n", desc)}
		var links = []string{}
		if item.Image != nil {
			msgs = append(msgs, message.Image(item.Image.URL))
			links = append(links, item.Image.URL)
		}
		for _, enclosure := range item.Enclosures {
			if strings.HasPrefix(enclosure.Type, "image/") && !slices.Contains(links, enclosure.URL) {
				msgs = append(msgs, message.Image(enclosure.URL))
				links = append(links, enclosure.URL)
			}
		}
		return nil, msgs
	case 5:
		err, s := getTemplate(db, int64(res.Gid), res.Feed)
		if err != nil {
			return err, nil
		}
		err, msg := templateRender(s, item, feed)
		if err != nil {
			return err, nil
		}
		return nil, message.UnescapeCQCodeText(msg)
	default:
	}
	return errors.New("unknown render type"), nil
}
func findRss(ctx *zero.Ctx, db *sql.Sqlite, link string) (*rssInfo, error) {
	var res = &rssInfo{}
	return res, db.Find("group_rss", res, fmt.Sprintf("where Feed like '%s' and Gid = %d", link, ctx.Event.GroupID))
}
func findRssById(ctx *zero.Ctx, db *sql.Sqlite, id int) (*rssInfo, error) {
	var res = &rssInfo{}
	return res, db.Find("group_rss", res, fmt.Sprintf("where Id = %d and Gid = %d", id, ctx.Event.GroupID))
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

func renderRssImage(item *gofeed.Item, client *http.Client) (error, []byte) {
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
