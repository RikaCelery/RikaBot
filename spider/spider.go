// Package spider 基于 https://shindanmaker.com 的测定小功能
package spider

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FloatTech/floatbox/process"
	"image"
	"image/jpeg"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/FloatTech/zbputils/control"

	"github.com/FloatTech/floatbox/file"
	sqlite "github.com/FloatTech/sqlite"
	"github.com/corona10/goimagehash"
	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/mattn/go-runewidth"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"

	"github.com/FloatTech/ZeroBot-Plugin/utils"
)

// LastValidatedRKey 缓存上次有效的 rkey
var LastValidatedRKey = ""

type row struct {
	ID   int // pk
	Name string
}

type forwardInfo struct {
	ID      int
	Images  []fileStruct
	Videos  []fileStruct
	Links   []string
	Magnets []string
	RawJSON string
}
type fileStruct struct {
	Path      string
	ImageSize int64
	VideoHash string
	URL       string
}

func (f *fileStruct) Identity() string {
	if f.VideoHash != "" {
		return f.VideoHash
	} else if f.ImageSize != 0 {
		return strconv.FormatInt(f.ImageSize, 10)
	}
	return f.Path
}

type downloadPathInfo struct {
	URL   string
	Type  string
	Hash  string
	PHash string
	Path  string
}

func removeDuplicates(slice []string) []string {
	result := make([]string, 0, len(slice))
	encountered := make(map[string]bool)

	for _, str := range slice {
		if !encountered[str] {
			encountered[str] = true
			result = append(result, str)
		}
	}

	return result
}

func removeDuplicatesFile(slice []fileStruct) []fileStruct {
	result := make([]fileStruct, 0, len(slice))
	encountered := make(map[string]bool)

	for _, str := range slice {
		if !encountered[str.Path] {
			encountered[str.Path] = true
			result = append(result, str)
		}
	}

	return result
}

var caches = map[string]bool{}
var client = http.Client{}

func downloadImageFromURL(imageURL string, oc chan string) error {
	var retryed = 0

retry:
	if retryed != 0 {
		// 解析 URL
		parsedURL, _ := url.Parse(imageURL)
		// 获取查询参数
		queryParams := parsedURL.Query()
		if LastValidatedRKey != "" && LastValidatedRKey != queryParams.Get("rkey") {
			queryParams.Set("rkey", LastValidatedRKey)
			logrus.Debugf("update image url %s", imageURL)
		}
		// 构造新的 URL
		parsedURL.RawQuery = queryParams.Encode()
		imageURL = parsedURL.String()
	}
	// if ok, _ := caches[imageURL]; ok {
	//	return nil
	//}
	// 发送HTTP请求下载图片
	resp, err := client.Get(imageURL)
	if err != nil {
		if errors.Is(err, http.ErrHandlerTimeout) {
			goto retry
		}
		return err
	}
	// fmt.Printf("%v\n", resp.Header)
	defer resp.Body.Close()

	buf := make([]byte, 1024*1024)
	l, _ := resp.Body.Read(buf)

	// 读取前几个字节以确定文件类型
	fileExt, err := getFileTypeFromBytes(buf[:l])
	if err != nil {
		retryed++
		if retryed >= 3 {
			return err
		}
		goto retry
	}

	// 解析 URL
	parsedURL, _ := url.Parse(imageURL)
	// 获取查询参数
	queryParams := parsedURL.Query()
	// 获取 rkey 参数的值
	LastValidatedRKey = queryParams.Get("rkey")

	// 创建一个 MD5 哈希对象
	hasher := md5.New()

	// 读取响应体并写入到哈希对象，同时保存到临时文件
	tempFile, err := os.CreateTemp("tmp", "_downloaded_image_"+fileExt)
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	// 复制响应体到哈希对象和临时文件
	multiWriter := io.MultiWriter(hasher, tempFile)
	_, err = multiWriter.Write(buf[:l])
	if err != nil {
		return err
	}
	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return err
	}

	// 计算文件的 MD5 哈希值
	hash := hasher.Sum(nil)
	md5Str := hex.EncodeToString(hash)

	// 构建最终文件名
	finalFileName := filepath.Join("tmp", fmt.Sprintf("%s%s", md5Str, fileExt))
	tempFile.Close()
	// 将临时文件重命名为最终文件
	if err := os.Rename(tempFile.Name(), finalFileName); err != nil {
		return err
	}

	if oc != nil {
		oc <- finalFileName
	}
	logrus.Infoln("[spider] Image saved as:", finalFileName)
	caches[imageURL] = true
	return nil
}
func downloadVideoFromURL(videoURL string, oc chan string) error {
	// 发送HTTP请求下载图片
	resp, err := http.Get(videoURL)
	if err != nil {
		return fmt.Errorf("error downloading video: %v, url=%s", err, videoURL) //nolint:forbidigo
	}
	// fmt.Printf("%v\n", resp.Header)
	defer resp.Body.Close()

	buf := make([]byte, 1024*1024)
	l, _ := resp.Body.Read(buf)

	// 读取前几个字节以确定文件类型
	fileExt, err := getFileTypeFromBytes(buf[:l])
	if err != nil {
		return err
	}

	// 创建一个 MD5 哈希对象
	hasher := md5.New()

	// 读取响应体并写入到哈希对象，同时保存到临时文件
	tempFile, err := os.CreateTemp("videotmp", "_downloaded_video_"+fileExt)
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	// 复制响应体到哈希对象和临时文件
	multiWriter := io.MultiWriter(hasher, tempFile)
	_, err = multiWriter.Write(buf[:l])
	if err != nil {
		logrus.Warnln("[spider] failed to write video:", err)
		return err
	}
	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return err
	}

	// 计算文件的 MD5 哈希值
	hash := hasher.Sum(nil)
	md5Str := hex.EncodeToString(hash)

	// 构建最终文件名
	finalFileName := filepath.Join("videotmp", fmt.Sprintf("%s%s", md5Str, fileExt))
	tempFile.Close()
	// 将临时文件重命名为最终文件
	if err := os.Rename(tempFile.Name(), finalFileName); err != nil {
		return err
	}

	if oc != nil {
		oc <- finalFileName
	}
	logrus.Infoln("[spider] Video saved as:", finalFileName)
	caches[videoURL] = true
	return nil
}
func downloadFileFromURL(fileURL string, fileExt string, oc chan string) error {
	// 发送HTTP请求下载图片
	resp, err := http.Get(fileURL)
	if err != nil {
		return fmt.Errorf("error downloading video: %v, url=%s", err, fileURL) //nolint:forbidigo
	}
	// fmt.Printf("%v\n", resp.Header)
	defer resp.Body.Close()

	buf := make([]byte, 1024*1024)
	l, _ := resp.Body.Read(buf)

	// 创建一个 MD5 哈希对象
	hasher := md5.New()

	// 读取响应体并写入到哈希对象，同时保存到临时文件
	tempFile, err := os.CreateTemp("filetmp", "_downloaded_file_"+fileExt)
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	// 复制响应体到哈希对象和临时文件
	multiWriter := io.MultiWriter(hasher, tempFile)
	_, err = multiWriter.Write(buf[:l])
	if err != nil {
		logrus.Warnln("[spider] failed to write file:", err)
		return err
	}
	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return err
	}

	// 计算文件的 MD5 哈希值
	hash := hasher.Sum(nil)
	md5Str := hex.EncodeToString(hash)

	// 构建最终文件名
	finalFileName := filepath.Join("filetmp", fmt.Sprintf("%s%s", md5Str, fileExt))
	tempFile.Close()
	// 将临时文件重命名为最终文件
	if err := os.Rename(tempFile.Name(), finalFileName); err != nil {
		return err
	}

	if oc != nil {
		oc <- finalFileName
	}
	logrus.Infoln("[spider] Video saved as:", finalFileName)
	caches[fileURL] = true
	return nil
}
func downloadGroupFileFromURL(fileURL string, dest string) error {
	// 发送HTTP请求下载图片
	resp, err := http.Get(fileURL)
	if err != nil {
		return fmt.Errorf("error downloading group file: %v, url=%s", err, fileURL) //nolint:forbidigo
	}
	// fmt.Printf("%v\n", resp.Header)
	defer resp.Body.Close()

	buf := make([]byte, 1024*1024)
	l, _ := resp.Body.Read(buf)

	// 读取响应体并写入到哈希对象，同时保存到临时文件
	tempFile, err := os.CreateTemp("gpfile", "_downloaded_file_")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	// 复制响应体到哈希对象和临时文件
	multiWriter := tempFile
	_, err = multiWriter.Write(buf[:l])
	if err != nil {
		logrus.Warnln("[spider] failed to write file:", err)
		return err
	}
	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return err
	}

	tempFile.Close()
	// 将临时文件重命名为最终文件
	dir, _ := filepath.Split(dest)
	if !utils.Exists(dir) {
		os.MkdirAll(dir, 0755)
	}
	if err := os.Rename(tempFile.Name(), dest); err != nil {
		return err
	}
	caches[fileURL] = true
	return nil
}

// getFileTypeFromBytes 根据文件的前几个字节确定文件类型和扩展名
func getFileTypeFromBytes(fileType []byte) (string, error) {
	mime := mimetype.Detect(fileType)
	// 根据文件头部字节判断文件类型
	// contentType := http.DetectContentType(fileType)
	fileExt := mime.Extension()
	switch mime.String() {
	case "image/jpeg":
		fileExt = ".jpg"
	case "image/png":
		fileExt = ".png"
	case "image/gif":
		fileExt = ".gif"
	case "image/bmp":
		fileExt = ".bmp"
	case "video/mp4":
		fileExt = ".mp4"
	case "application/json":
		return "", fmt.Errorf("unsupported file type: %s %s", mime.String(), string(fileType)) //nolint:forbidigo
	default:
		return "", fmt.Errorf("unsupported file type: %s(%s)", mime.String(), fileExt) //nolint:forbidigo
	}

	return fileExt, nil
}

var downloadDB *sqlite.Sqlite

func storeImageHashToDB(u, path, phash, hash string) error {
	p, _ := url.Parse(u)
	query := p.Query()
	query.Del("rkey")
	p.RawQuery = query.Encode()
	err := downloadDB.Insert("downloads", &downloadPathInfo{
		URL:   p.String(),
		Type:  "image",
		Hash:  hash,
		PHash: phash,
		Path:  path,
	})
	return err
}
func getImageHashFromFile(filename string) (phash string, md5hash string, err error) {
	readFile, err := os.ReadFile(filename)
	if err != nil {
		return "", "", err
	}
	img, _, err := image.Decode(bytes.NewReader(readFile))
	if err != nil {
		return "", "", err
	}
	phash_, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return "", "", err
	}
	sum := md5.New()
	sum.Write(readFile)
	md5hash = hex.EncodeToString(sum.Sum(nil))
	return fmt.Sprintf("%016x", phash_.GetHash()), md5hash, nil
}

//nolint:unparam
func getHashStored(u string) (phash string, hash string, err error) {
	p, _ := url.Parse(u)
	query := p.Query()
	query.Del("rkey")
	p.RawQuery = query.Encode()
	u = p.String()
	if !downloadDB.CanFind("downloads", "where URL = ?", u) {
		return "", "", errors.New("file not downloaded")
	}
	info := downloadPathInfo{}
	err = downloadDB.Find("downloads", &info, "where URL = ?", u)
	if err != nil {
		return "", "", err
	}
	phash = info.PHash
	hash = info.Hash
	return
}
func hasHashStored(u string) bool {
	p, _ := url.Parse(u)
	query := p.Query()
	query.Del("rkey")
	p.RawQuery = query.Encode()
	u = p.String()
	return downloadDB.CanFind("downloads", "where URL = ?", u)
}

// Init 初始化爬虫
func Init() {
	db := &sqlite.Sqlite{DBPath: "spider.db"}
	downloadDB = &sqlite.Sqlite{DBPath: "downloadDB.db"}
	err := db.Open(time.Minute)
	if err != nil {
		panic(err)
	}
	err = downloadDB.Open(time.Minute)
	if err != nil {
		panic(err)
	}

	err = downloadDB.Create("downloads", &downloadPathInfo{})
	if err != nil {
		panic(err)
	}

	err = db.Create("infos", &row{})
	if err != nil {
		panic(err)
	}
	_, err = db.DB.Exec(`
create table if not exists forward_hash
(
    forward_id text not null,
    hash       text not null,
    constraint forward_hash_pk
        primary key (hash, forward_id)
);

create table if not exists digests
(
    hash   text not null
        constraint digests_pk
            primary key,
    digest text not null
);
create table if not exists file_name_map
(
    hash   text not null
        constraint file_name_map_pk
            primary key,
    name text not null
);

`)
	if err != nil {
		panic(err)
	}
	replyRegExp := regexp.MustCompile(`\[CQ:reply,id=(-?\d+)].*`)
	((*control.Matcher)(zero.OnMessage(zero.SuperUserPermission)).SetName("spider.set_summary")).Handle(func(ctx *zero.Ctx) {
		plainText := strings.TrimSpace(ctx.ExtractPlainText())
		logrus.Debugf("[spider] msg %s", plainText)
		if !strings.HasPrefix(plainText, zero.BotConfig.CommandPrefix+"digest") {
			return
		}
		if !replyRegExp.MatchString(ctx.MessageString()) {
			logrus.Debugf("[spider] regex not match %s", ctx.MessageString())
			return
		}
		digest := strings.TrimSpace(plainText[len(zero.BotConfig.CommandPrefix+"digest"):])
		messageID := replyRegExp.FindStringSubmatch(ctx.MessageString())[1]
		msg := ctx.GetMessage(messageID)
		if msg.MessageID.ID() == 0 {
			ctx.Send("[ERROR]:id为0，未找到消息")
			return
		}
		forwardID := msg.Elements[0].Data["id"]
		if !db.CanQuery("select * from forward_hash where forward_id = ?", forwardID) {
			ctx.Send("[ERROR]:数据库中不存在该转发消息，请先尝试重新转发")
			return
		}
		hashStr, err := sqlite.Query[string](db, "select hash from forward_hash where forward_id = ?", forwardID)
		if err != nil {
			ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
			return
		}
		_, err = db.DB.Exec("replace into digests(hash,digest) values (?,?)", hashStr, digest)
		if err != nil {
			ctx.Send(fmt.Sprintf("[ERROR]:%v", err))
			return
		}
		logrus.Infof("[spider] 设置成功, %s => %s", hashStr, digest)
		ctx.Send("[INFO]:设置成功")
	})
	onMessage := zero.OnMessage()
	onMessage.NoTimeout = true
	((*control.Matcher)(onMessage.SetPriority(-3)).SetName("spider.spider")).Handle(func(ctx *zero.Ctx) {
		preprocess(ctx)
		handle(db, ctx)
	})
	zero.On("notice/group_upload").Handle(func(ctx *zero.Ctx) {
		netName := ctx.Event.RawEvent.Get("file.name").String()
		netURL := ctx.Event.RawEvent.Get("file.url").String()
		go func() {
			err := os.Mkdir("filetmp", 0750)
			if err != nil && !errors.Is(err, os.ErrExist) {
				logrus.Warningf("[spider] failed to mkdir `filetmp`: %v", err)
				return
			}
			oc := make(chan string, 1)
			defer close(oc)
			err = downloadFileFromURL(netURL, filepath.Ext(netName), oc)
			if err != nil {
				logrus.Warningf("[spider] failed to download file: %v", err)
				return
			}
			_, err = db.DB.Exec(`replace into file_name_map(hash,name) values (?,?)`, <-oc, netName)
			if err != nil {
				logrus.Warningf("[spider] failed to insert file: %v", err)
				return
			}
		}()
		logrus.Infof("[fileStruct] %s %s \n%s", netName, netURL, ctx.Event.RawEvent.String())
		if strings.HasSuffix(strings.ToLower(netName), ".apk") || strings.HasSuffix(strings.ToLower(netName), ".apk.1") || strings.HasPrefix(strings.ToLower(netName), "base.apk") || strings.HasPrefix(strings.ToLower(netName), "base(1).apk") || strings.HasPrefix(strings.ToLower(netName), "base(2).apk") || strings.HasPrefix(strings.ToLower(netName), "base(3).apk") {
			err := file.DownloadTo(netURL, netName)
			if err != nil {
				panic(err)
			}
			icon, pkgName, cn, manifest, err := utils.ParseApk(netName)
			if err != nil {
				panic(err)
			}
			buf := &bytes.Buffer{}
			if icon != nil {
				_ = jpeg.Encode(buf, *icon, &jpeg.Options{Quality: 60})
			}
			sdkMin, err := manifest.SDK.Min.Int32()
			if err != nil {
				sdkMin = -1
			}
			sdkTarget, err := manifest.SDK.Target.Int32()
			if err != nil {
				sdkTarget = -1
			}
			vname, err := manifest.VersionName.String()
			if err != nil {
				vname = "解析失败"
			}
			vcode, err := manifest.VersionCode.Int32()
			if err != nil {
				vcode = -9
			}
			var fileSize string
			stat, err := os.Stat(netName)
			if err == nil {
				fileSize = humanize.BigIBytes(big.NewInt(stat.Size()))
			}
			var msgs message.Message = []message.Segment{
				message.Text(fmt.Sprintf(
					"安装包:\n%s\n包名:\n%s\n版本名称:%s\n版本号:%d\nSDK:[%d,%d(target)]\nSize:%s",
					cn,
					runewidth.Truncate(pkgName, 30, "..."),
					runewidth.Truncate(vname, 30, "..."),
					vcode,
					sdkMin,
					sdkTarget,
					fileSize,
				)),
			}
			if icon != nil {
				msgs = append(msgs, message.ImageBytes(buf.Bytes()))
			} else {
				msgs = append(msgs, message.Text("\n图图炸了！"))
			}
			ctx.SendGroupMessage(ctx.Event.GroupID, msgs)
			_ = os.Remove(netName)
		}
	})

	zero.OnCommand("download", zero.SuperUserPermission, zero.CheckArgs(func(ctx *zero.Ctx, args []string) bool {

		if len(args) == 0 {
			for _, result := range ctx.GetGroupList().Array() {
				println(result.String())
				args = append(args, result.Get("group_id").String())
			}
		}
		send := ""
		gps := make([]int64, 0)
		for _, arg := range args {
			groupID, _ := strconv.ParseInt(arg, 10, 64)
			gps = append(gps, groupID)
			if ctx.GetGroupInfo(groupID, false).ID == 0 {
				send += fmt.Sprintf("%d 不存在\n", groupID)
			}
		}
		if send != "" {
			ctx.SendChain(message.Text(send))
			return false
		}
		ctx.State["groups"] = gps
		return true
	})).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		gps := ctx.State["groups"].([]int64)
		utils.ParallelMap(gps, 20, func(gp int64) (string, error) {
			downloadGroup(ctx, gp)
			return "", nil
		})
	}).NoTimeout = true
}

type vfile struct {
	GID          int64
	RemotePath   string
	Uploader     int64
	UploaderName string
	UploadTime   int64
	FileSize     int64
	Busid        int64
	ID           string
}

func (v vfile) UName() string {
	dir, fname := filepath.Split(v.RemotePath)
	fname = fmt.Sprintf("%s_%s", strconv.FormatInt(v.UploadTime, 10), fname)
	return filepath.Join(strconv.FormatInt(v.GID, 10), dir, fname)
}
func (v vfile) Download(ctx *zero.Ctx) (string, error) {
	logrus.Infof("[spider] download %d %d %s => %s", v.GID, v.Busid, v.ID, v.UName())
	fileURL := ctx.GetGroupFileURL(v.GID, v.Busid, v.ID)
	if fileURL == "" {
		return "", errors.New("failed to get group file url")
	}
	dest := filepath.Join("gpfile", v.UName())
	if utils.Exists(dest) {
		return dest, nil
	}
	err := downloadGroupFileFromURL(fileURL, dest)
	if err != nil {
		return "", err
	}
	return dest, nil
}

func (v vfile) DownloadURL(ctx *zero.Ctx) string {
	return ctx.GetGroupFileURL(v.GID, v.Busid, v.ID)
}
func downloadGroup(ctx *zero.Ctx, gp int64) {
	ret := ctx.GetGroupRootFiles(gp)
	downloadQueue := make([]vfile, 0)
	files := ret.Get("files").Array()
	for _, result := range files {
		downloadQueue = append(downloadQueue, vfile{
			GID:          gp,
			RemotePath:   result.Get("file_name").Str,
			Uploader:     result.Get("uploader").Int(),
			UploaderName: result.Get("uploader_name").Str,
			UploadTime:   result.Get("upload_time").Int(),
			FileSize:     result.Get("file_size").Int(),
			Busid:        result.Get("busid").Int(),
			ID:           result.Get("file_id").String(),
		})
	}
	folders := ret.Get("folders").Array()
	R := utils.ParallelMap(folders, 8, func(folder gjson.Result) ([]vfile, error) {
		files := ctx.GetGroupFilesByFolder(gp, folder.Get("folder_id").String()).Get("files").Array()
		downloadQueue := make([]vfile, 0)
		for _, result := range files {
			downloadQueue = append(downloadQueue, vfile{
				GID:          gp,
				RemotePath:   filepath.Join(folder.Get("folder_name").String(), result.Get("file_name").Str),
				Uploader:     result.Get("uploader").Int(),
				UploaderName: result.Get("uploader_name").Str,
				UploadTime:   result.Get("upload_time").Int(),
				FileSize:     result.Get("file_size").Int(),
				Busid:        result.Get("busid").Int(),
				ID:           result.Get("file_id").String(),
			})
		}
		return downloadQueue, nil
	})
	for _, folder := range R {
		if folder.Err != nil {
			logrus.Errorf("error")
			continue
		}
		for _, v := range *folder.Ret {
			downloadQueue = append(downloadQueue, v)
		}

	}
	//if !utils.Exists("gpfile/ok") {
	//	os.MkdirAll("gpfile/ok", 0777)
	//}
	ctx.Send(fmt.Sprintf("准备下载(%d)，共%d", gp, len(downloadQueue)))
	join := make([]map[string]string, 0)
	for i, v := range downloadQueue {
		join = append(join, map[string]string{
			"URL":  v.DownloadURL(ctx),
			"path": v.UName(),
		})
		if (i+1)%32 == 0 {
			j, _ := json.Marshal(join)
			join = make([]map[string]string, 0)
			_, err := http.Post("http://127.0.0.1:5637/download", "application/json", strings.NewReader(string(j)))
			if err != nil {
				logrus.Infoln(err.Error())
			}
		}
	}
	j, _ := json.Marshal(join)
	_, err := http.Post("http://127.0.0.1:5637/download", "application/json", strings.NewReader(string(j)))
	if err != nil {
		logrus.Infoln(err.Error())
	}
	//utils.ParallelMap(downloadQueue, 8, func(v vfile) (string, error) {
	//	path := fmt.Sprintf("gpfile/ok/%s", v.UName())
	//	dir, _ := filepath.Split(path)
	//	if !utils.Exists(dir) {
	//		os.MkdirAll(dir, 0777)
	//	}
	//	if utils.Exists(path) {
	//		os.Create(path)
	//		return path, nil
	//	}
	//	_, err := v.Download(ctx)
	//	if err != nil {
	//		logrus.Errorln(err)
	//		return path, nil
	//	}
	//	os.Create(path)
	//	return path, nil
	//})
	ctx.Send(fmt.Sprintf("下载成功，共%d", len(downloadQueue)))
}

func (f *fileStruct) getHash(download bool) string {
	switch {
	case f.ImageSize != 0:
		switch {
		case hasHashStored(f.URL):
			phash, _, _ := getHashStored(f.URL)
			return phash
		case download:
			var oc = make(chan string, 1)
			defer close(oc)
			err := downloadImageFromURL(f.URL, oc)
			if err == nil {
				filename := <-oc
				phash, hash, err := getImageHashFromFile(filename)
				if err == nil {
					err := storeImageHashToDB(f.URL, filename, phash, hash)
					if err != nil {
						logrus.Errorf("[spider] store ImageHash failed %v", err)
					}
				}
				return phash
			}
			logrus.Errorf("[spider] getHash, download filed: %v", err)
			return f.Identity()
		default:
			logrus.Errorf("[spider] hash not found for %s", f.URL)
			return f.Identity()
		}
	case f.VideoHash != "":
		return f.Identity()
	default:
		logrus.Warnf("both image size and video hash is empty")
		return "[error]"
	}
}
func hashForward(textContent string, fInfo *forwardInfo, download bool) string {
	var addi = make([]string, 1)
	addi[0] = fmt.Sprintf("%d", len(fInfo.Images))
	// TODO
	for _, img := range fInfo.Images {
		if hasHashStored(img.URL) {
			phash, _, _ := getHashStored(img.URL)
			addi = append(addi, phash)
		} else {
			addi = append(addi, img.getHash(download))
		}
	}
	addi = append(addi, fmt.Sprintf("%d", len(fInfo.Videos)))
	for _, video := range fInfo.Videos {
		addi = append(addi, video.Identity())
	}
	// TODO
	// for _, fileStruct := range fInfo.Files {
	//	textContent += "\nv:" + fileStruct.Identity()
	//}
	return hashForward2(textContent, addi)
}
func hashForward2(textContent string, addi []string) string {
	hash := md5.New()
	for _, addiHash := range addi {
		textContent += "\n" + addiHash
	}
	logrus.Debugf("[spider] hashing \n%s", textContent)
	hash.Write([]byte(textContent))
	return hex.EncodeToString(hash.Sum(nil))
}

func handle(db *sqlite.Sqlite, ctx *zero.Ctx) {
	var images []fileStruct
	var videos []fileStruct
	var links []string
	var magnets []string
	var textContent = ""
	var _, forwardMsg = ctx.State["forward_id"]
	if forwardMsg {
		textContent = ctx.State["forward_id"].(string)
	}
	var forwardID string
	processed := ctx.State["DATA"].(utils.Mapped[message.Segment])
	for _, msg := range processed {
		if msg.Err != nil {
			logrus.Errorf("[spider] error: %s", msg.Err)
			continue
		}
		msgType := msg.Ret.Type
		switch msgType {
		case "forward":
		case "text":
			s := msg.Ret.Data["text"]
			textContent += s
			var urlRegexp = regexp.MustCompile(`https?://(www\.)?[-a-zA-Z0-9@:%._+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_+.~#?&/=]*)`)
			allString := urlRegexp.FindAllString(s, -1)
			links = append(links, allString...)
			var magnetRegexp = regexp.MustCompile(`([0-9a-zA-Z]{40}|[0-9a-zA-Z]{32})`)
			allString = magnetRegexp.FindAllString(s, -1)
			for _, s2 := range allString {
				magnets = append(magnets, fmt.Sprintf("magnet:?xt=urn:btih:%s", s2))
			}
		case "image":
			images = append(images,
				fileStruct{
					Path: msg.Ret.Data["filename"],
					URL:  msg.Ret.Data["url"],
					//ImageSize: 10,
					VideoHash: msg.Ret.Data["phash"],
					//ImageSize: result.Get("data.ImageSize").Int(),
				})
		case "video":
			videos = append(videos,
				fileStruct{
					Path: msg.Ret.Data["url"],
					URL:  msg.Ret.Data["url"],
				})

		case "reply":
		case "at":

		default:
			logrus.Debugf("[spider] unsupport message type: %v", msgType)
			continue
		}
	}

	images = removeDuplicatesFile(images)
	videos = removeDuplicatesFile(videos)
	links = removeDuplicates(links)
	magnets = removeDuplicates(magnets)

	var send = ""
	var forwardHash = ""
	info := &forwardInfo{
		ID:      int(ctx.Event.MessageID.(int64)),
		Images:  images,
		Links:   links,
		Magnets: magnets,
		Videos:  videos,
		RawJSON: string(ctx.Event.NativeMessage),
	}
	if forwardMsg {
		forwardHash = hashForward(textContent, info, true)
		if db.CanQuery("select * from digests where hash = ?", forwardHash) {
			query, err := sqlite.Query[string](db, "select digest from digests where hash = ? ", forwardHash)
			if err == nil {
				logrus.Infof("[spider] digest %s", query)
				send += fmt.Sprintf("省流: \n%s\n", query)
			} else {
				logrus.Errorf("[spider] %v", err)
			}
		}
		if !db.CanQuery("select * from forward_hash(forward_id,hash) values (?,?)", forwardID, forwardHash) {
			_, err := db.DB.Exec("replace into forward_hash(forward_id,hash) values (?,?)", forwardID, forwardHash)
			if err != nil {
				logrus.Errorf("[spider] replace forward_hash failed %v", err)
				return
			}
		}
	}
	if len(info.Links) != 0 {
		send += fmt.Sprintf("%d条链接 ", len(info.Links))
	}
	if len(info.Videos) != 0 {
		send += fmt.Sprintf("%d条视频 ", len(info.Videos))
	}
	if len(info.Images) != 0 {
		send += fmt.Sprintf("%d条图片 ", len(info.Images))
	}
	if len(info.Magnets) != 0 {
		send += "🧲:\n" + strings.Join(info.Magnets, "\n")
	}
	if forwardMsg && !strings.Contains(send, "省流") && (len(info.Images) > 30 || len(info.Videos) > 30 || len(info.Magnets) > 0) {
		process.SleepAbout1sTo2s()
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("省流(%s): %s", forwardHash, send)))
	}
	if len(images) == 0 && len(videos) == 0 && len(magnets) == 0 {
		return
	}
	marshal, _ := json.Marshal(info)

	if forwardMsg {
		var r row
		err := db.Find("infos", &r, fmt.Sprintf("where ID=%d", info.ID))
		if err == nil {
			err := db.Del("infos", fmt.Sprintf("where ID=%d", info.ID))
			if err != nil {
				logrus.Errorf("[spider] del failed %v", err)
				return
			}
		}
		err = db.Insert("infos", &row{
			ID:   info.ID,
			Name: string(marshal),
		})
		if err != nil {
			logrus.Errorf("[spider] insert failed %v", err)
			return
		}
	}
	if !forwardMsg && len(magnets) > 0 {
		var r row
		err := db.Find("infos", &r, fmt.Sprintf("where ID=%d", info.ID))
		if err == nil {
			err := db.Del("infos", fmt.Sprintf("where ID=%d", info.ID))
			if err != nil {
				logrus.Errorf("[spider] del failed %v", err)
				return
			}
		}
		err = db.Insert("infos", &row{
			ID:   info.ID,
			Name: string(marshal),
		})
		if err != nil {
			logrus.Errorf("[spider] insert failed %v", err)
			return
		}
	}
}

func parse(result gjson.Result, filter []string, callback func(res gjson.Result)) {
	t := result.Get("type").String()
	switch t {
	case "MultiMsgEntity":
		for _, r := range result.Get("Chains").Array() {
			parse(r, filter, callback)
		}
		return
	case "TextEntity":
		// logrus.Debugf("[spider] Text: %s\n", result.Get("Text").String())

		for i := range filter {
			if filter[i] == t {
				callback(result)
			}
		}
		return
	case "VideoEntity":
		// logrus.Debugf("[spider] ImageSize: %s\n", result.Get("ImageSize").Int())
		for i := range filter {
			if filter[i] == t {
				callback(result)
			}
		}

	case "ImageEntity":
		// logrus.Debugf("[spider] ImageSize: %s\n", result.Get("ImageSize").Int())
		for i := range filter {
			if filter[i] == t {
				callback(result)
			}
		}
		return
	case "MessageChain":
		for _, r := range result.Get("MessageChain").Array() {
			parse(r, filter, callback)
		}
		return
	case "XmlEntity":
		return
	}
	// logrus.Debugf("[spider] type: %s\n", t)
}
