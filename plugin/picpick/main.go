// Package picpick 简单图片收藏
package picpick

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FloatTech/floatbox/math"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/corona10/goimagehash"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/shell"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/ZeroBot-Plugin/spider"
)

var db = &picPickDB{}

type picPickDB struct {
	DB   *sql.Sqlite
	lock *sync.Mutex
}
type fileEntity struct {
	ID    int
	Path  string
	PHash string
	Md5   string
}
type picWithFilePath struct {
	FileID int `db:"file_id"`
	PicID  int `db:"pic_id"`
	Path   string
}
type picEntity struct {
	ID     int
	FileID int
}

//nolint:unused
type picTag struct {
	ID    int
	Tag   string
	PicID int
}

var (
	errPicNotFound = errors.New("pic not found")
)

func (p *picPickDB) createDB(engine *control.Engine) error {
	if p.DB == nil {
		p.DB = &sql.Sqlite{
			DBPath: engine.DataFolder() + "picpick.db",
		}
		p.lock = &sync.Mutex{}
	}
	err := p.DB.Open(time.Hour)
	if err != nil {
		return err
	}
	_, err = p.DB.DB.Exec(`
create table if not exists files
(
    ID    INTEGER not null
        primary key autoincrement,
    Path  TEXT    not null,
    PHash TEXT    not null,
    md5   TEXT    not null
);

create unique index if not exists files_Path_uindex
    on files (Path);

create table if not exists pics
(
    ID     INTEGER not null,
    FileID INTEGER not null,
    primary key (ID, FileID)
);

create table if not exists tags
(
    id    INTEGER not null
        primary key autoincrement,
    Tag   TEXT    not null,
    PicID INTEGER not null
);

create unique index if not exists tags_PicId_Tag_uindex
    on tags (PicID, Tag);


`)
	if err != nil {
		return err
	}
	return nil
}
func (p *picPickDB) getPicByFileID(fileID int) (*picEntity, error) {
	if !db.DB.CanFind("pics", "where FileID = "+strconv.Itoa(fileID)) {
		return nil, errPicNotFound
	}
	var find = &picEntity{}
	err := p.DB.Find("pics", find, "where FileID = "+strconv.Itoa(fileID))
	if err != nil {
		return nil, err
	}
	return find, nil
}
func (p *picPickDB) delPicByID(picID int) error {
	if !db.DB.CanFind("pics", "where ID = "+strconv.Itoa(picID)) {
		return errPicNotFound
	}
	err := p.DB.Del("pics", "where ID = ?", picID)
	if err != nil {
		return err
	}
	err = p.DB.Del("tags", "where PicID = ?", picID)
	if err != nil {
		return err
	}
	return nil
}
func (p *picPickDB) delFileByPicID(fileID int) error {
	if !db.DB.CanFind("pics", "where ID = "+strconv.Itoa(fileID)) {
		return errPicNotFound
	}
	pic, err := sql.Find[picEntity](p.DB, "pics", "where ID = ?", fileID)
	if err != nil {
		return err
	}
	err = p.DB.Del("files", "where ID = ?", pic.FileID)
	if err != nil {
		return err
	}
	err = p.DB.Del("pics", "where ID = ?", pic.ID)
	if err != nil {
		return err
	}
	err = p.DB.Del("tags", "where PicID = ?", pic.ID)
	if err != nil {
		return err
	}
	return nil
}
func (p *picPickDB) getAllTags() ([]string, error) {
	find, err := sql.QueryAll[string](p.DB, "select distinct Tag from tags")
	if err != nil {
		return nil, err
	}
	var ret = make([]string, len(find))
	for i, v := range find {
		ret[i] = *v
	}
	return ret, nil
}
func (p *picPickDB) getFilesByTags(tags []string, count int, random bool) ([]*picWithFilePath, error) {
	buf := strings.Builder{}
	for i := range tags {
		if i == 0 {
			buf.WriteString(fmt.Sprintf(`select distinct f.ID as file_id, t0.PicID as pic_id,f.Path as Path from tags t%d
         join pics p on p.ID = t0.PicID
         join files f on f.ID = p.FileID`, i))
			continue
		}
		buf.WriteString(fmt.Sprintf(`
         join tags t%d on t0.PicID = t%d.PicID`, i, i))
	}
	for i := range tags {
		if i == 0 {
			buf.WriteString(fmt.Sprintf(`
where t%d.Tag = ? `, i))
			continue
		}
		buf.WriteString(fmt.Sprintf(`
  and t%d.Tag = ? `, i))
	}
	if random {
		buf.WriteString(`
order by random()`)
	}
	if count > 0 {
		buf.WriteString(fmt.Sprintf(`
limit %d`, count))
	}
	logrus.Debug("[picpick] execute sql: ", buf.String())
	var args = make([]interface{}, len(tags))
	for i, tag := range tags {
		args[i] = tag
	}
	if !p.DB.CanQuery(buf.String(), args...) {
		return make([]*picWithFilePath, 0), nil
	}
	row, err := sql.QueryAll[picWithFilePath](p.DB, buf.String(), args...)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// func (p *picPickDB) getPicsByFileId(FileID int) (pics []*picEntity, err error) {
// }
func (p *picPickDB) getFilesByPHash(phash string, distance int) (files []*fileEntity, err error) {
	if !p.DB.CanFind("files", "") {
		return make([]*fileEntity, 0), nil
	}

	var ret []*fileEntity
	var t = &fileEntity{}
	parseUint, err := strconv.ParseUint(phash, 16, 64)
	if err != nil {
		return nil, err
	}
	hash := goimagehash.NewImageHash(parseUint, goimagehash.PHash)
	err = p.DB.FindFor("files", t, "", func() error {
		parseUint, _ := strconv.ParseUint(t.PHash, 16, 64)
		hash2 := goimagehash.NewImageHash(parseUint, goimagehash.PHash)
		i, err := hash.Distance(hash2)
		if err != nil {
			return err
		}
		if i <= distance {
			ret = append(ret, &fileEntity{
				ID:    t.ID,
				Path:  t.Path,
				PHash: t.PHash,
				Md5:   t.Md5,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

//
// func (p *picPickDB) getFilesByMd5(md5 string) (file *fileEntity, err error) {
//}

func (p *picPickDB) InsertFile(path string, phash string, md5 string) (int, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	var id = 0
	var err error
	if p.DB.CanFind("files", "") {
		id, err = sql.Query[int](p.DB, "select id from files order by id desc limit 1;")
		if err != nil {
			return 0, err
		}
	}
	id++
	_, err = p.DB.DB.Exec(`replace into files (ID,Path, PHash, md5) values (?,?,?,?);`, id, path, phash, md5)
	if err != nil {
		return 0, err
	}
	return id, nil
}
func (p *picPickDB) InsertPic(fileID int) (int, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	var id = 0
	var err error
	if p.DB.CanFind("pics", "") {
		id, err = sql.Query[int](p.DB, "select id from pics order by id desc limit 1;")
		if err != nil {
			return 0, err
		}
	}
	id++
	_, err = p.DB.DB.Exec(`insert into pics (ID,FileID) values (?,?);`, id, fileID)
	if err != nil {
		return 0, err
	}
	return id, nil
}
func (p *picPickDB) InsertPicTags(tags []string, picID int) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	tx, err := p.DB.DB.Begin()
	if err != nil {
		return err
	}
	for _, tag := range tags {
		_, err = tx.Exec("replace into tags(PicID, Tag) VALUES (?,?)", picID, tag)
		if err != nil {
			return err
		}
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()
	return nil
}

func init() {
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "图片收藏",
		Help: `
- {prefix}pic a <标签1> <标签2> <标签3>... + 图片或回复图片    收藏这张图片，并给图片打上tag
- {prefix}pic get <标签1> <标签2> <标签3>...   返回一张符合所有tag的图片
- {prefix}pic ls 列出所有tag
- {prefix}pic ls <标签名字> 列出tag对应的所有图片
- {prefix}pic rm <id> 删除id的图片
- {prefix}pic file rm <id> 删除某个id的文件，如果该文件被其他图片引用，则一并删除`,
		PrivateDataFolder: "picPick",
	})
	err := db.createDB(engine)
	client := &http.Client{}
	if err != nil {
		panic(err)
	}
	engine.OnCommand("pic ls").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		tags, err := db.getAllTags()
		if err != nil {
			ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
			return
		}
		ctx.Send(strings.Join(tags, "\n"))
	})
	engine.OnMessage(func(ctx *zero.Ctx) bool {
		text := ctx.ExtractPlainText()
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, zero.BotConfig.CommandPrefix+"pic a") {
			cmdString := text[len(zero.BotConfig.CommandPrefix+"pic a"):]
			ctx.State["args"] = cmdString
			return true
		}
		return false
	}, zero.AdminPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		tags := shell.Parse(ctx.State["args"].(string))
		for _, segment := range ctx.Event.Message {
			if segment.Type == "reply" {
				msg := ctx.GetMessage(segment.Data["id"])
				for _, element := range msg.Elements {
					if element.Type == "image" {
						link, ok := element.Data["file"]
						if !ok || link == "" {
							link = element.Data["url"]
						}
						if strings.HasPrefix(link, "http://gchat.qpic.cn/gchatpic_new") || link == "" { // lgr bug
							continue
						}
						fileName, hashMd5, phash, err := downloadToMd5File(client, engine.DataFolder(), link)
						if err != nil {
							marshal, _ := json.Marshal(element)
							println(string(marshal))
							ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
							return
						}
						files, err := db.getFilesByPHash(phash, 2)
						if err != nil || len(files) > 1 {
							files = make([]*fileEntity, 0)
						}
						var fileID int
						if len(files) == 0 {
							fileID, err = db.InsertFile(fileName, phash, hashMd5)
							if err != nil {
								ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
								return
							}
						} else {
							fileID = files[0].ID
						}
						var picID int
						entity, err := db.getPicByFileID(fileID)
						if err == nil {
							picID = entity.ID
						} else {
							picID, err = db.InsertPic(fileID)
							if err != nil {
								ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
								return
							}
						}
						err = db.InsertPicTags(tags, picID)
						if err != nil {
							ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
							return
						}
						ctx.Send(fmt.Sprintf("√ -> %d", picID))
					}
				}
			}
		}
	})
	engine.OnCommand("pic get").Limit(ctxext.LimitByGroup).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		files, err := db.getFilesByTags(shell.Parse(ctx.State["args"].(string)), 1, true)
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		if len(files) == 0 {
			ctx.SendChain(message.Text("[ERROR]:没有找到图片"))
			return
		}
		var msgs []message.MessageSegment
		for _, file := range files {
			msgs = append(msgs, message.Text(file.PicID))
			readFile, err := os.ReadFile(file.Path)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			msgs = append(msgs, message.ImageBytes(readFile))
		}
		ctx.Send(msgs)
	})
	engine.OnCommand("pic rm", zero.AdminPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		picID, err := strconv.Atoi(ctx.State["args"].(string))
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		err = db.delPicByID(picID)
		if errors.Is(err, errPicNotFound) {
			ctx.SendChain(message.Text("[ERROR]:没有找到图片"))
			return
		}
		ctx.Send("删除成功")
	})
	engine.OnCommand("pic file rm", zero.AdminPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		picID, err := strconv.Atoi(ctx.State["args"].(string))
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		err = db.delFileByPicID(picID)
		if errors.Is(err, errPicNotFound) {
			ctx.SendChain(message.Text("[ERROR]:没有找到图片"))
			return
		}
		ctx.Send("删除成功")
	})
}

func downloadToMd5File(client *http.Client, folder string, link string) (string, string, string, error) {
	parsedURL, _ := url.Parse(link)

	// 获取查询参数
	queryParams := parsedURL.Query()
	if spider.LastValidatedRKey != "" {
		queryParams.Set("rkey", spider.LastValidatedRKey)
	}
	// 构造新的 URL
	parsedURL.RawQuery = queryParams.Encode()
	imageURL := parsedURL.String()
	logrus.Infof("update image url %s", imageURL)
	resp, err := client.Get(imageURL)
	if err != nil {
		return "", "", "", err
	}
	buffer := &bytes.Buffer{}
	defer resp.Body.Close()
	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		return "", "", "", err
	}
	hashMd5 := md5.New()
	hashMd5.Write(buffer.Bytes())
	md5Hex := hex.EncodeToString(hashMd5.Sum(nil))
	img, format, err := image.Decode(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		logrus.Errorf("decode image error %v %v\n%v", err.Error(), imageURL, hex.Dump(buffer.Bytes()[:math.Min(buffer.Len(), 20)]))
		return "", "", "", err
	}

	phash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return "", "", "", err
	}
	newFileName := fmt.Sprintf("%s%s.%s", folder, md5Hex, format)
	open, err := os.OpenFile(newFileName, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return "", "", "", err
	}
	_, err = open.Write(buffer.Bytes())
	if err != nil {
		return "", "", "", err
	}
	open.Close()
	buffer.Reset()
	return newFileName, md5Hex, fmt.Sprintf("%016x", phash.GetHash()), nil
}
