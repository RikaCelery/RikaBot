// Package spider 基于 https://shindanmaker.com 的测定小功能
package spider

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FloatTech/ZeroBot-Plugin/utils"
	"github.com/FloatTech/floatbox/file"
	sqlite "github.com/FloatTech/sqlite"
	"github.com/corona10/goimagehash"
	"github.com/gabriel-vasile/mimetype"
	"github.com/mattn/go-runewidth"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var LastValidatedRKey = ""

type row struct {
	ID   int // pk
	Name string
}

type forwardInfo struct {
	Id      int
	Images  []fileStruct
	Videos  []fileStruct
	Links   []string
	Magnets []string
	RawJson string
}
type fileStruct struct {
	Path      string
	ImageSize int64
	VideoHash string
	Url       string
}

func (f *fileStruct) Identity() string {
	if f.VideoHash != "" {
		return f.VideoHash
	} else if f.ImageSize != 0 {
		return strconv.FormatInt(f.ImageSize, 10)
	} else {
		return f.Path
	}
}

type downloadPathInfo struct {
	Url   string
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
	//if ok, _ := caches[imageURL]; ok {
	//	return nil
	//}
	// 发送HTTP请求下载图片
	resp, err := client.Get(imageURL)
	if err != nil {
		return fmt.Errorf("Error downloading image: %v", err)
	}
	//fmt.Printf("%v\n", resp.Header)
	defer resp.Body.Close()

	buf := make([]byte, 1024*1024)
	l, _ := resp.Body.Read(buf)

	// 读取前几个字节以确定文件类型
	fileExt, err := getFileTypeFromBytes(buf[:l])
	if err != nil {
		retryed++
		if retryed == 3 {
			return fmt.Errorf("error determining file type: %v", err)
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
	tempFile, err := os.CreateTemp("tmp", "_downloaded_image_*"+fileExt)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// 复制响应体到哈希对象和临时文件
	multiWriter := io.MultiWriter(hasher, tempFile)
	multiWriter.Write(buf[:l])
	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return fmt.Errorf("failed to read image: %v", err)
	}

	// 计算文件的 MD5 哈希值
	hash := hasher.Sum(nil)
	md5Str := hex.EncodeToString(hash)

	// 构建最终文件名
	finalFileName := filepath.Join("tmp", fmt.Sprintf("%s%s", md5Str, fileExt))
	tempFile.Close()
	// 将临时文件重命名为最终文件
	if err := os.Rename(tempFile.Name(), finalFileName); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
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
		return fmt.Errorf("Error downloading video: %v, url=%s", err, videoURL)
	}
	//fmt.Printf("%v\n", resp.Header)
	defer resp.Body.Close()

	buf := make([]byte, 1024*1024)
	l, _ := resp.Body.Read(buf)

	// 读取前几个字节以确定文件类型
	fileExt, err := getFileTypeFromBytes(buf[:l])
	if err != nil {
		return fmt.Errorf("error determining file type: %v", err)
	}

	// 创建一个 MD5 哈希对象
	hasher := md5.New()

	// 读取响应体并写入到哈希对象，同时保存到临时文件
	tempFile, err := os.CreateTemp("tmp", "_downloaded_video_*"+fileExt)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// 复制响应体到哈希对象和临时文件
	multiWriter := io.MultiWriter(hasher, tempFile)
	multiWriter.Write(buf[:l])
	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return fmt.Errorf("failed to write video: %v", err)
	}

	// 计算文件的 MD5 哈希值
	hash := hasher.Sum(nil)
	md5Str := hex.EncodeToString(hash)

	// 构建最终文件名
	finalFileName := filepath.Join("tmp", fmt.Sprintf("%s%s", md5Str, fileExt))
	tempFile.Close()
	// 将临时文件重命名为最终文件
	if err := os.Rename(tempFile.Name(), finalFileName); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	if oc != nil {
		oc <- finalFileName
	}
	logrus.Infoln("[spider] Video saved as:", finalFileName)
	caches[videoURL] = true
	return nil
}

// getFileTypeFromBytes 根据文件的前几个字节确定文件类型和扩展名
func getFileTypeFromBytes(fileType []byte) (string, error) {
	mime := mimetype.Detect(fileType)
	// 根据文件头部字节判断文件类型
	//contentType := http.DetectContentType(fileType)
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
		return "", fmt.Errorf("unsupported file type: %s %s", mime.String(), string(fileType))
	default:
		return "", fmt.Errorf("unsupported file type: %s", mime.String())
	}

	return fileExt, nil
}

var downloadDb *sqlite.Sqlite

func storeImageHashToDB(Url, path, phash, hash string) error {
	p, _ := url.Parse(Url)
	query := p.Query()
	query.Del("rkey")
	p.RawQuery = query.Encode()
	err := downloadDb.Insert("downloads", &downloadPathInfo{
		Url:   p.String(),
		Type:  "image",
		Hash:  hash,
		PHash: phash,
		Path:  path,
	})
	return err
}
func getImageHashFromFile(filename string) (string, string, error) {
	readFile, err := os.ReadFile(filename)
	if err != nil {
		return "", "", err
	}
	img, _, err := image.Decode(bytes.NewReader(readFile))
	if err != nil {
		return "", "", err
	}
	phash, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return "", "", err
	}
	sum := md5.New()
	sum.Write(readFile)
	md5hash := hex.EncodeToString(sum.Sum(nil))
	return fmt.Sprintf("%016x", phash.GetHash()), md5hash, nil
}
func getHashStored(Url string) (phash string, hash string, err error) {
	p, _ := url.Parse(Url)
	query := p.Query()
	query.Del("rkey")
	p.RawQuery = query.Encode()
	Url = p.String()
	if !downloadDb.CanFind("downloads", "where Url = ?", Url) {
		return "", "", errors.New("file not downloaded")
	}
	info := downloadPathInfo{}
	err = downloadDb.Find("downloads", &info, "where Url = ?", Url)
	if err != nil {
		return "", "", err
	}
	phash = info.PHash
	hash = info.Hash
	return
}
func hasHashStored(Url string) bool {
	p, _ := url.Parse(Url)
	query := p.Query()
	query.Del("rkey")
	p.RawQuery = query.Encode()
	Url = p.String()
	return downloadDb.CanFind("downloads", "where Url = ?", Url)
}
func Init() {
	db := &sqlite.Sqlite{DBPath: "spider.db"}
	downloadDb = &sqlite.Sqlite{DBPath: "downloadDb.db"}
	err := db.Open(time.Minute)
	if err != nil {
		panic(err)
	}
	err = downloadDb.Open(time.Minute)
	if err != nil {
		panic(err)
	}

	err = downloadDb.Create("downloads", &downloadPathInfo{})
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

`)
	if err != nil {
		panic(err)
	}
	replyRegExp := regexp.MustCompile(`\[CQ:reply,id=(-?\d+)].*`)
	zero.OnMessage(zero.SuperUserPermission).Handle(func(ctx *zero.Ctx) {
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
		messageId := replyRegExp.FindStringSubmatch(ctx.MessageString())[1]
		msg := ctx.GetMessage(messageId)
		if msg.MessageId.ID() == 0 {
			ctx.Send(fmt.Sprintf("[ERROR]:id为0，未找到消息"))
			return
		}
		forwardId := msg.Elements[0].Data["id"]
		if !db.CanQuery("select * from forward_hash where forward_id = ?", forwardId) {
			ctx.Send(fmt.Sprintf("[ERROR]:数据库中不存在该转发消息，请先尝试重新转发"))
			return
		}
		hashStr, err := sqlite.Query[string](db, "select hash from forward_hash where forward_id = ?", forwardId)
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
		ctx.Send(fmt.Sprintf("[INFO]:设置成功"))
	})
	zero.OnMessage().Handle(func(ctx *zero.Ctx) {
		Handle(db, ctx)
	})
}
func (f *fileStruct) getHash(download bool) string {
	if f.ImageSize != 0 {
		if hasHashStored(f.Url) {
			phash, _, _ := getHashStored(f.Url)
			return phash
		} else if download {
			var oc = make(chan string, 1)
			defer close(oc)
			err := downloadImageFromURL(f.Url, oc)
			if err == nil {
				filename := <-oc
				phash, hash, err := getImageHashFromFile(filename)
				if err == nil {
					err := storeImageHashToDB(f.Url, filename, phash, hash)
					if err != nil {
						logrus.Errorf("[spider] store ImageHash failed %v", err)
					}
				}
				return phash
			} else {
				logrus.Errorf("[spider] getHash, download filed: %v", err)
				return f.Identity()
			}
		} else {
			logrus.Errorf("[spider] hash not found for %s", f.Url)
			return f.Identity()
		}
	} else if f.VideoHash != "" {
		return f.Identity()
	} else {
		panic("both image size and video hash is empty")
	}
}
func hashForward(textContent string, fInfo *forwardInfo, download bool) string {
	var addi []string
	addi = append(addi, fmt.Sprintf("%d", len(fInfo.Images)))
	//TODO
	//addi = append(addi, strconv.FormatInt(int64(len(fInfo.Files)),10))
	for _, img := range fInfo.Images {
		if hasHashStored(img.Url) {
			phash, _, _ := getHashStored(img.Url)
			addi = append(addi, phash)
		} else {
			addi = append(addi, img.getHash(download))
		}
	}
	addi = append(addi, fmt.Sprintf("%d", len(fInfo.Videos)))
	for _, video := range fInfo.Videos {
		addi = append(addi, video.Identity())
	}
	//TODO
	//for _, fileStruct := range fInfo.Files {
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

func Handle(db *sqlite.Sqlite, ctx *zero.Ctx) {
	var images []fileStruct
	var videos []fileStruct
	var links []string
	var magnets []string
	var textContent = ""
	res := gjson.ParseBytes(ctx.Event.NativeMessage)
	var forwardMsg = false
	var forwardId string

	for _, result := range res.Array() {
		msgType := result.Get("type").String()
		switch msgType {
		case "forward":
			forwardMsg = true
			forwardId = result.Get("data.id").String()
			parse(result.Get("data.json"),
				[]string{
					"TextEntity",
					"ImageEntity",
					"VideoEntity",
				}, func(res gjson.Result) {
					if res.Get("type").String() == "TextEntity" {
						s := res.Get("Text").String()
						textContent += s
						var urlRegxp, _ = regexp.Compile("https?://(www\\.)?[-a-zA-Z0-9@:%._+~#=]{1,256}\\.[a-zA-Z0-9()]{1,6}\\b([-a-zA-Z0-9()@:%_+.~#?&/=]*)")
						allString := urlRegxp.FindAllString(s, -1)
						for _, s2 := range allString {
							links = append(links, s2)
						}
						var magnetRegexp, _ = regexp.Compile("([0-9a-zA-Z]{40}|[0-9a-zA-Z]{32})")
						allString = magnetRegexp.FindAllString(s, -1)
						for _, s2 := range allString {
							magnets = append(magnets, fmt.Sprintf("magnet:?xt=urn:btih:%s", s2))
						}
					} else if res.Get("type").String() == "ImageEntity" {
						s, _ := url.Parse(res.Get("ImageUrl").String())
						values := s.Query()
						values.Set("appid", "1407")
						s.RawQuery = values.Encode()
						images = append(images,
							fileStruct{
								Path:      res.Get("FilePath").String(),
								Url:       s.String(),
								ImageSize: res.Get("ImageSize").Int(),
							})
					} else {
						videos = append(videos,
							fileStruct{
								Path: res.Get("FilePath").String(),
								Url:  res.Get("VideoUrl").String(),
							})

					}
				})
		case "text":
			textContent += result.Get("data.text").String()
			continue
		case "image":
			u := result.Get("data.url").String()
			err := downloadImageFromURL(u, nil)
			if err != nil {
				logrus.Errorf("[spider] image downlaod failed: %v", err.Error())
				continue
			}
		case "video":
			u := result.Get("data.url").String()
			err := downloadVideoFromURL(u, nil)
			if err != nil {
				logrus.Errorf("[spider] video download filed: %v", err.Error())
				continue
			}
			continue
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
		Id:      int(ctx.Event.MessageID.(int64)),
		Images:  images,
		Links:   links,
		Magnets: magnets,
		Videos:  videos,
		RawJson: string(ctx.Event.NativeMessage),
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
		if db.CanQuery("select * from forward_hash(forward_id,hash) values (?,?)", forwardId, forwardHash) {
		} else {
			db.DB.Exec("replace into forward_hash(forward_id,hash) values (?,?)", forwardId, forwardHash)
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
	if len(send) > 0 {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("%s %s", forwardHash, send)))
	}
	if len(images) == 0 && len(videos) == 0 && len(magnets) == 0 {
		return
	}

	marshal, _ := json.Marshal(info)
	var r row
	err := db.Find("infos", &r, fmt.Sprintf("where ID=%d", info.Id))
	if err == nil {
		db.Del("infos", fmt.Sprintf("where ID=%d", info.Id))
	}
	db.Insert("infos", &row{
		ID:   info.Id,
		Name: string(marshal),
	})
	// download
	if len(info.Links) == 0 {
		goto sendLinkEnd
	}
	{
		//var base64ed []string
		//md5Hash := md5.Sum([]byte(strings.Join(info.Links, "\n")))
		//abs, _ := filepath.Abs(fmt.Sprintf("%x_links.txt", md5Hash))
		//if _, err := os.Stat(abs); err != nil {
		//	goto sendLinkEnd
		//}
		//for _, link := range info.Links {
		//	// avoid content audit
		//	base64ed = append(base64ed, base64.StdEncoding.EncodeToString([]byte(link)))
		//}
		//if len(base64ed) != 0 {
		//	f, _ := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY, 0644)
		//	_, _ = f.WriteString(strings.Join(base64ed, "\n"))
		//	f.Close()
		//	// not upload
		//}
	}
sendLinkEnd:
	var oc = make(chan string, len(info.Images))
	if len(info.Images) != 0 {
		//client := http.Client{}
		var wg = sync.WaitGroup{}
		var imgFiles []string
		for _, image := range info.Images {
			wg.Add(1)
			image := image
			go func() {
				defer func() {
					wg.Done()
					if r := recover(); r != nil {
					}
				}()
				err := downloadImageFromURL(image.Url, oc)
				if err != nil {
					logrus.Warnln("[spider] download Failed", image, err.Error())
					return
				}
			}()
		}
		wg.Wait()
		close(oc)
		imgFiles = make([]string, 0)
		for s := range oc {
			imgFiles = append(imgFiles, s)
		}
		imgFiles = removeDuplicates(imgFiles)
		if len(imgFiles) == 0 {
			logrus.Warn("[spider] No imgs.")
			return
		}
		sort.Strings(imgFiles)
		//md5Hash := md5.Sum([]byte(strings.Join(imgFiles, "\n")))
		//imgFileListPath := fmt.Sprintf("%x.images.txt", md5Hash)
		//imgFileList, err := os.OpenFile(imgFileListPath, os.O_CREATE|os.O_WRONLY, 0644)
		//if err != nil {
		//	panic(err)
		//	return
		//}
		//_, err = imgFileList.WriteString(strings.Join(imgFiles, "\n"))
		//imgFileList.Close()
		//if err != nil {
		//	panic(err)
		//	return
		//}
		//
		//imgArchiveAbs, _ := filepath.Abs(fmt.Sprintf("pack.%x.imgs.7z", md5Hash))
		//cmd :=
		//	exec.Command("7z", "a", "-y", "-p1145141919810", "-mhe=on", imgArchiveAbs, fmt.Sprintf("@%s", imgFileListPath))
		//cmd.Stdout = os.Stdout
		//cmd.Stderr = os.Stderr
		//err = cmd.Start()
		//if err != nil {
		//	cmd.Wait()
		//	err = os.Remove(imgFileListPath)
		//	if err != nil {
		//		println(err.Error())
		//	}
		//	//upload no
		//	//if ctx.Event.GroupID == 564828920 || ctx.Event.GroupID == 839852697 || ctx.Event.GroupID == 924075421 || ctx.Event.GroupID == 946855395 {
		//	//	r := ctx.UploadThisGroupFile(imgArchiveAbs, fmt.Sprintf("img包(%d)#%x.7z", len((imgFiles)), md5Hash), "")
		//	//	if r.RetCode != 0 {
		//	//		logrus.Warn("returns", r.RetCode)
		//	//	}
		//	//}
		//}
	}
	{
		var wg = sync.WaitGroup{}
		for _, video := range info.Videos {
			video := video
			wg.Add(1)
			go func() {
				defer wg.Done()
				os.Mkdir("videotmp", 0750)
				fname := path.Join("videotmp", video.Path)
				if _, err := os.Stat(fname); err == nil {
					logrus.Infoln("[spider] exist", video)
					return

				}

				resp, err2 := client.Get(video.Url)
				if err2 != nil {
					logrus.Warnln("[spider] ", err2)
					return
				}
				logrus.Infoln("[spider] download", video)
				file, err2 := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0644)
				if err2 != nil {
					return
				}
				//s, _ := filepath.Abs(fname)
				_, err := io.Copy(file, resp.Body)
				if err != nil {
					file.Close()
					os.Remove(fname)
					logrus.Warnln("[spider] download Failed", video)
					return
				}
				file.Close()
				logrus.Infoln("[spider] download OK", video)
			}()

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
		//logrus.Debugf("[spider] Text: %s\n", result.Get("Text").String())

		for i := range filter {
			if filter[i] == t {
				callback(result)
			}
		}
		return
	case "VideoEntity":
		//logrus.Debugf("[spider] ImageSize: %s\n", result.Get("ImageSize").Int())
		for i := range filter {
			if filter[i] == t {
				callback(result)
			}
		}

	case "ImageEntity":
		//logrus.Debugf("[spider] ImageSize: %s\n", result.Get("ImageSize").Int())
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
	//logrus.Debugf("[spider] type: %s\n", t)
}
