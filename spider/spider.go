// Package spider 基于 https://shindanmaker.com 的测定小功能
package spider

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	sqlite "github.com/FloatTech/sqlite"
	"github.com/gabriel-vasile/mimetype"
	"net/url"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
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
	Images  []file
	Videos  []file
	Links   []string
	Magnets []string
	RawJson string
}
type file struct {
	Path string
	Url  string
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

func removeDuplicatesFile(slice []file) []file {
	result := make([]file, 0, len(slice))
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

func downloadImageFromURL(imageURL string, oc chan string) error {
	var retryed = 0

retry:
	if retryed != 0 {
		// 解析 URL
		parsedURL, _ := url.Parse(imageURL)

		// 获取查询参数
		queryParams := parsedURL.Query()
		queryParams.Set("rkey", LastValidatedRKey)
		// 构造新的 URL
		parsedURL.RawQuery = queryParams.Encode()
		imageURL = parsedURL.String()
		logrus.Infof("update image url %s", imageURL)
	}
	//if ok, _ := caches[imageURL]; ok {
	//	return nil
	//}
	// 发送HTTP请求下载图片
	resp, err := http.Get(imageURL)
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
	fmt.Println("Image saved as:", finalFileName)
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
	fmt.Println("Video saved as:", finalFileName)
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
	default:
		return "", fmt.Errorf("unsupported file type: %s", mime.String())
	}

	return fileExt, nil
}

func Init() {
	db := &sqlite.Sqlite{DBPath: "spider.db"}
	err := db.Open(time.Minute)
	if err != nil {
		panic(err)
	}

	err = db.Create("infos", &row{})
	if err != nil {
		panic(err)
	}
	zero.OnMessage().Handle(func(ctx *zero.Ctx) {
		//println(string(ctx.Event.NativeMessage))
		var images = []file{}
		var videos = []file{}
		var links = []string{}
		var magnets = []string{}
		res := gjson.ParseBytes(ctx.Event.NativeMessage)
		for _, result := range res.Array() {
			msgType := result.Get("type").String()
			switch msgType {
			case "forward":
				//t := result.Get("json.type").String()
				parse(result.Get("data.json"),
					[]string{
						"TextEntity",
						"ImageEntity",
						"VideoEntity",
					}, func(res gjson.Result) {
						if res.Get("type").String() == "TextEntity" {
							s := res.Get("Text").String()
							println(s)
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
							println(res.Get("ImageUrl").String())
							images = append(images,
								file{
									Path: res.Get("FilePath").String(),
									Url:  res.Get("ImageUrl").String(),
								})
						} else {
							println(res.Get("ImageUrl").String())
							videos = append(videos,
								file{
									Path: res.Get("FilePath").String(),
									Url:  res.Get("VideoUrl").String(),
								})

						}
					})
			case "text":
				continue
			case "image":
				u := result.Get("data.url").String()
				err := downloadImageFromURL(u, nil)
				if err != nil {
					logrus.Errorln(err.Error())
					continue
				}
			case "video":
				u := result.Get("data.url").String()
				err := downloadVideoFromURL(u, nil)
				if err != nil {
					logrus.Errorln(err.Error())
					continue
				}
				continue
			case "reply":
			case "at":

			default:
				logrus.Infoln(msgType)
				continue
			}
		}
		images = removeDuplicatesFile(images)
		videos = removeDuplicatesFile(videos)
		links = removeDuplicates(links)
		magnets = removeDuplicates(magnets)
		if len(images) == 0 && len(videos) == 0 && len(magnets) == 0 {
			return
		}
		info := forwardInfo{
			Id:      int(ctx.Event.MessageID.(int64)),
			Images:  images,
			Links:   links,
			Magnets: magnets,
			Videos:  videos,
			RawJson: string(ctx.Event.NativeMessage),
		}
		marshal, _ := json.Marshal(info)
		var r row
		err = db.Find("infos", &r, fmt.Sprintf("where ID=%d", info.Id))

		if err == nil {
			db.Del("infos", fmt.Sprintf("where ID=%d", info.Id))
		}
		db.Insert("infos", &row{
			ID:   info.Id,
			Name: string(marshal),
		})
		var send = ""
		if len(info.Links)+len(info.Magnets)+len(info.Videos)+len(info.Images) != 0 {
			send = fmt.Sprintf("省流:%d条链接,%d条🧲,%d条视频,%d条图片\n", len(info.Links), len(info.Magnets), len(info.Videos), len(info.Images))
		}
		if len(info.Magnets) != 0 {
			send += "🧲:\n" + strings.Join(info.Magnets, "\n")
		}
		logrus.Infof("ctx.Event.GroupID %d\n", ctx.Event.GroupID)
		if len(send) > 0 {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(send))
		}
		// download
		if len(info.Links) == 0 {
			goto sendLinkEnd
		}
		{
			var base64ed []string
			md5Hash := md5.Sum([]byte(strings.Join(info.Links, "\n")))
			abs, _ := filepath.Abs(fmt.Sprintf("%x_links.txt", md5Hash))
			if _, err := os.Stat(abs); err != nil {
				goto sendLinkEnd
			}
			for _, link := range info.Links {
				// avoid content audit
				base64ed = append(base64ed, base64.StdEncoding.EncodeToString([]byte(link)))
			}
			if len(base64ed) != 0 {
				f, _ := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY, 0644)
				_, _ = f.WriteString(strings.Join(base64ed, "\n"))
				f.Close()
				// not upload
			}
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
						logrus.Warnln("download Failed", image, err.Error())
						return
					}
					logrus.Infoln("download OK", image)
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
				logrus.Warn("No imgs.")
				return
			}
			sort.Strings(imgFiles)
			md5Hash := md5.Sum([]byte(strings.Join(imgFiles, "\n")))
			imgFileListPath := fmt.Sprintf("%x.images.txt", md5Hash)
			imgFileList, err := os.OpenFile(imgFileListPath, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				panic(err)
				return
			}
			_, err = imgFileList.WriteString(strings.Join(imgFiles, "\n"))
			imgFileList.Close()
			if err != nil {
				panic(err)
				return
			}

			imgArchiveAbs, _ := filepath.Abs(fmt.Sprintf("pack.%x.imgs.7z", md5Hash))
			cmd :=
				exec.Command("7z", "a", "-y", "-p1145141919810", "-mhe=on", imgArchiveAbs, fmt.Sprintf("@%s", imgFileListPath))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Start()
			if err != nil {
				cmd.Wait()
				err = os.Remove(imgFileListPath)
				if err != nil {
					println(err.Error())
				}
				//upload no
				//if ctx.Event.GroupID == 564828920 || ctx.Event.GroupID == 839852697 || ctx.Event.GroupID == 924075421 || ctx.Event.GroupID == 946855395 {
				//	r := ctx.UploadThisGroupFile(imgArchiveAbs, fmt.Sprintf("img包(%d)#%x.7z", len((imgFiles)), md5Hash), "")
				//	if r.RetCode != 0 {
				//		logrus.Warn("returns", r.RetCode)
				//	}
				//}
			}
		}
		{
			client := http.Client{}
			var wg = sync.WaitGroup{}
			for _, video := range info.Videos {
				video := video
				wg.Add(1)
				go func() {
					defer wg.Done()
					os.Mkdir("videotmp", 0750)
					fname := path.Join("videotmp", video.Path)
					if _, err := os.Stat(fname); err == nil {
						logrus.Infoln("exist", video)
						return

					}

					resp, err2 := client.Get(video.Url)
					if err2 != nil {
						logrus.Warn(err2)
						return
					}
					logrus.Infoln("download", video)
					file, err2 := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0644)
					if err2 != nil {
						return
					}
					//s, _ := filepath.Abs(fname)
					_, err := io.Copy(file, resp.Body)
					if err != nil {
						file.Close()
						os.Remove(fname)
						logrus.Warnln("download Failed", video)
						return
					}
					file.Close()
					logrus.Infoln("download OK", video)
				}()

			}
		}
	})
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
		//logrus.Debugf("Text: %s\n", result.Get("Text").String())

		for i := range filter {
			if filter[i] == t {
				callback(result)
			}
		}
		return
	case "VideoEntity":
		//logrus.Debugf("ImageSize: %s\n", result.Get("ImageSize").Int())
		for i := range filter {
			if filter[i] == t {
				callback(result)
			}
		}

	case "ImageEntity":
		//logrus.Debugf("ImageSize: %s\n", result.Get("ImageSize").Int())
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
	//logrus.Debugf("type: %s\n", t)

}
