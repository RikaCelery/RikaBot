package picpick

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	sql "github.com/FloatTech/sqlite"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/corona10/goimagehash"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/shell"
	"github.com/wdvxdr1123/ZeroBot/message"
	"image"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var db = &picPickDB{}

type picPickDB struct {
	Db   *sql.Sqlite
	lock *sync.Mutex
}
type fileEntity struct {
	Id    int
	Path  string
	PHash string
	Md5   string
}
type picWithFilePath struct {
	FileId int `db:"file_id"`
	PicId  int `db:"pic_id"`
	Path   string
}
type picEntity struct {
	Id     int
	FileId int
}
type picTag struct {
	Id    int
	Tag   string
	PicId int
}

var (
	errPicNotFound = fmt.Errorf("pic not found")
)

func (p *picPickDB) createDb(engine *control.Engine) error {
	if p.Db == nil {
		p.Db = &sql.Sqlite{
			DBPath: engine.DataFolder() + "picpick.db",
		}
		p.lock = &sync.Mutex{}
	}
	err := p.Db.Open(time.Hour)
	if err != nil {
		return err
	}
	_, err = p.Db.DB.Exec(`
create table if not exists files
(
    Id    INTEGER not null
        primary key autoincrement,
    Path  TEXT    not null,
    PHash TEXT    not null,
    md5   TEXT    not null
);

create unique index if not exists files_Path_uindex
    on files (Path);

create table if not exists pics
(
    Id     INTEGER not null,
    FileId INTEGER not null,
    primary key (Id, FileId)
);

create table if not exists tags
(
    id    INTEGER not null
        primary key autoincrement,
    Tag   TEXT    not null,
    PicId INTEGER not null
);

create unique index if not exists tags_PicId_Tag_uindex
    on tags (PicId, Tag);


`)
	if err != nil {
		return err
	}
	return nil
}
func (p *picPickDB) getPicByFileId(fileId int) (*picEntity, error) {
	if !db.Db.CanFind("pics", "where FileId = "+strconv.Itoa(fileId)) {
		return nil, errPicNotFound
	}
	var find = &picEntity{}
	err := p.Db.Find("pics", find, "where FileId = "+strconv.Itoa(fileId))
	if err != nil {
		return nil, err
	}
	return find, nil
}
func (p *picPickDB) delPicById(picId int) error {
	if !db.Db.CanFind("pics", "where Id = "+strconv.Itoa(picId)) {
		return errPicNotFound
	}
	err := p.Db.Del("pics", "where Id = ?", picId)
	if err != nil {
		return err
	}
	err = p.Db.Del("tags", "where PicId = ?", picId)
	if err != nil {
		return err
	}
	return nil
}
func (p *picPickDB) delFileByPicId(fileId int) error {
	if !db.Db.CanFind("pics", "where Id = "+strconv.Itoa(fileId)) {
		return errPicNotFound
	}
	pic, err := sql.Find[picEntity](p.Db, "pics", "where Id = ?", fileId)
	if err != nil {
		return err
	}
	err = p.Db.Del("files", "where Id = ?", pic.FileId)
	if err != nil {
		return err
	}
	err = p.Db.Del("pics", "where Id = ?", pic.Id)
	if err != nil {
		return err
	}
	err = p.Db.Del("tags", "where PicId = ?", pic.Id)
	if err != nil {
		return err
	}
	return nil
}
func (p *picPickDB) getAllTags() ([]string, error) {
	find, err := sql.QueryAll[string](p.Db, "select distinct Tag from tags")
	if err != nil {
		return nil, err
	}
	var ret []string
	for _, v := range find {
		ret = append(ret, *v)
	}
	return ret, nil
}
func (p *picPickDB) getFilesByTags(tags []string, count int, random bool) ([]*picWithFilePath, error) {
	buf := strings.Builder{}
	for i, _ := range tags {
		if i == 0 {
			buf.WriteString(fmt.Sprintf(`select distinct f.Id as file_id, t0.PicId as pic_id,f.Path as Path from tags t%d
         join pics p on p.Id = t0.PicId
         join files f on f.Id = p.FileId`, i))
			continue
		}
		buf.WriteString(fmt.Sprintf(`
         join tags t%d on t0.PicId = t%d.PicId`, i, i))
	}
	for i, _ := range tags {
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
	var args []interface{}
	for _, tag := range tags {
		args = append(args, tag)
	}
	if !p.Db.CanQuery(buf.String(), args...) {
		return make([]*picWithFilePath, 0), nil
	}
	row, err := sql.QueryAll[picWithFilePath](p.Db, buf.String(), args...)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// func (p *picPickDB) getPicsByFileId(FileId int) (pics []*picEntity, err error) {
// }
func (p *picPickDB) getFilesByPHash(phash string, distance int) (files []*fileEntity, err error) {
	if !p.Db.CanFind("files", "") {
		return make([]*fileEntity, 0), nil
	}

	var ret []*fileEntity
	var t = &fileEntity{}
	parseUint, err := strconv.ParseUint(phash, 16, 64)
	if err != nil {
		return nil, err
	}
	hash := goimagehash.NewImageHash(parseUint, goimagehash.PHash)
	err = p.Db.FindFor("files", t, "", func() error {
		parseUint, _ := strconv.ParseUint(t.PHash, 16, 64)
		hash2 := goimagehash.NewImageHash(parseUint, goimagehash.PHash)
		i, err := hash.Distance(hash2)
		if err != nil {
			return err
		}
		if i <= distance {
			ret = append(ret, &fileEntity{
				Id:    t.Id,
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
//func (p *picPickDB) getFilesByMd5(md5 string) (file *fileEntity, err error) {
//}

func (p *picPickDB) InsertFile(path string, phash string, md5 string) (int, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	var id = 0
	var err error
	if p.Db.CanFind("files", "") {
		id, err = sql.Query[int](p.Db, "select id from files order by id desc limit 1;")
		if err != nil {
			return 0, err
		}
	}
	id++
	_, err = p.Db.DB.Exec(`replace into files (Id,Path, PHash, md5) values (?,?,?,?);`, id, path, phash, md5)
	if err != nil {
		return 0, err
	}
	return id, nil
}
func (p *picPickDB) InsertPic(fileId int) (int, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	var id = 0
	var err error
	if p.Db.CanFind("pics", "") {
		id, err = sql.Query[int](p.Db, "select id from pics order by id desc limit 1;")
		if err != nil {
			return 0, err
		}
	}
	id++
	_, err = p.Db.DB.Exec(`insert into pics (Id,FileId) values (?,?);`, id, fileId)
	if err != nil {
		return 0, err
	}
	return id, nil
}
func (p *picPickDB) InsertPicTags(tags []string, picId int) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	tx, err := p.Db.DB.Begin()
	if err != nil {
		return err
	}
	for _, tag := range tags {
		_, err = tx.Exec("replace into tags(PicId, Tag) VALUES (?,?)", picId, tag)
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
	err := db.createDb(engine)
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
						fileName, hashMd5, phash, err := downloadToMd5File(client, engine.DataFolder(), element.Data["url"])
						if err != nil {
							ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
							return
						}
						files, err := db.getFilesByPHash(phash, 2)
						if err != nil || len(files) > 1 {
							files = make([]*fileEntity, 0)
						}
						fileId := -1
						if len(files) == 0 {
							fileId, err = db.InsertFile(fileName, phash, hashMd5)
							if err != nil {
								ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
								return
							}
						} else {
							fileId = files[0].Id
						}
						var picId = -1
						entity, err := db.getPicByFileId(fileId)
						if err == nil {
							picId = entity.Id
						} else {
							picId, err = db.InsertPic(fileId)
							if err != nil {
								ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
								return
							}
						}
						err = db.InsertPicTags(tags, picId)
						if err != nil {
							ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
							return
						}
						ctx.Send(fmt.Sprintf("√ -> %d", picId))
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
			msgs = append(msgs, message.Text(file.PicId))
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
		picId, err := strconv.Atoi(ctx.State["args"].(string))
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		err = db.delPicById(picId)
		if errors.Is(err, errPicNotFound) {
			ctx.SendChain(message.Text("[ERROR]:没有找到图片"))
			return
		}
		ctx.Send("删除成功")

	})
	engine.OnCommand("pic file rm", zero.AdminPermission).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		picId, err := strconv.Atoi(ctx.State["args"].(string))
		if err != nil {
			ctx.SendChain(message.Text("[ERROR]:", err))
			return
		}
		err = db.delFileByPicId(picId)
		if errors.Is(err, errPicNotFound) {
			ctx.SendChain(message.Text("[ERROR]:没有找到图片"))
			return
		}
		ctx.Send("删除成功")

	})
}

func downloadToMd5File(client *http.Client, folder string, link string) (string, string, string, error) {
	resp, err := client.Get(link)
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
