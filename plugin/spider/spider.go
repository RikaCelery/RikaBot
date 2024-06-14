// Package spider 基于 https://shindanmaker.com 的测定小功能
package spider

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	sqlite "github.com/FloatTech/sqlite"
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
	"strconv"
	"strings"
	"sync"
	"time"

	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
)

type row struct {
	ID   int // pk
	Name string
}

type ForwardInfo struct {
	Id      int
	Images  []File
	Videos  []File
	Links   []string
	Magnets []string
	RawJson string
}
type File struct {
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

func removeDuplicatesFile(slice []File) []File {
	result := make([]File, 0, len(slice))
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

func downloadImageFromURL(imageURL string) error {
	if ok, _ := caches[imageURL]; ok {
		return nil
	}
	// 发送HTTP请求下载图片
	resp, err := http.Get(imageURL)
	if err != nil {
		return fmt.Errorf("Error downloading image: %v", err)
	}
	fmt.Printf("%v\n", resp.Header)
	defer resp.Body.Close()

	buf := make([]byte, 512)
	resp.Body.Read(buf)

	// 读取前几个字节以确定文件类型
	fileExt, err := getFileTypeFromBytes(buf)
	if err != nil {
		return fmt.Errorf("error determining file type: %v", err)
	}

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
	multiWriter.Write(buf)
	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		return fmt.Errorf("failed to read image: %v", err)
	}

	// 计算文件的 MD5 哈希值
	hash := hasher.Sum(nil)
	md5Str := hex.EncodeToString(hash)

	// 构建最终文件名
	finalFileName := filepath.Join("tmp", fmt.Sprintf("%s%s", md5Str, fileExt))

	// 将临时文件重命名为最终文件
	if err := os.Rename(tempFile.Name(), finalFileName); err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	fmt.Println("Image saved as:", finalFileName)
	caches[imageURL] = true
	return nil
}

// getFileTypeFromBytes 根据文件的前几个字节确定文件类型和扩展名
func getFileTypeFromBytes(fileType []byte) (string, error) {
	// 根据文件头部字节判断文件类型
	contentType := http.DetectContentType(fileType)
	fileExt := ""
	switch contentType {
	case "image/jpeg":
		fileExt = ".jpg"
	case "image/png":
		fileExt = ".png"
	case "image/gif":
		fileExt = ".gif"
	case "image/bmp":
		fileExt = ".bmp"
	default:
		return "", fmt.Errorf("unsupported file type: %s, hex: %s", contentType, hex.Dump(fileType))
	}

	return fileExt, nil
}

func init() {
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "spider",
		Help: "- 今天是什么少女[@xxx]\n" +
			"- 异世界转生[@xxx]\n" +
			"- 卖萌[@xxx]\n" +
			"- 今日老婆[@xxx]\n" +
			"- 黄油角色[@xxx]",
	})

	db := &sqlite.Sqlite{DBPath: "spider.db"}
	err := db.Open(time.Minute)
	if err != nil {
		panic(err)
	}

	err = db.Create("infos", &row{})
	if err != nil {
		panic(err)
	}
	engine.OnRegex(`^\[CQ:reply,id=(-?\d+)].*query`).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			// 删除需要查询的消息ID
			id, _ := strconv.Atoi(ctx.State["regex_matched"].([]string)[1])
			logrus.Debugf("query ID %d", id)
			var r row
			err = db.Find("infos", &r, fmt.Sprintf("where ID=%d", id))
			if err != nil {
				ctx.Send("NotFound🤔")
				return
			}
			var info ForwardInfo
			err := json.Unmarshal([]byte(r.Name), &info)
			if err != nil {
				println(err.Error())
				return
			}
			{
				var base64ed []string
				md5Hash := md5.Sum([]byte(strings.Join(info.Links, "\n")))
				abs, _ := filepath.Abs(fmt.Sprintf("%x_links_txt", md5Hash))
				for _, link := range info.Links {
					// avoid content audit
					base64ed = append(base64ed, base64.StdEncoding.EncodeToString([]byte(link)))
				}
				for _, link := range info.Magnets {
					// avoid content audit
					base64ed = append(base64ed, base64.StdEncoding.EncodeToString([]byte(link)))
				}
				if len(base64ed) != 0 {
					f, _ := os.OpenFile(abs, os.O_CREATE|os.O_WRONLY, 0644)
					_, _ = f.WriteString(strings.Join(base64ed, "\n"))
					f.Close()
					ctx.UploadThisGroupFile(abs, filepath.Base(abs), "/")
				}
			}
			//println(strings.Join(info.Images, "\n"))
			client := http.Client{}
			var wg = sync.WaitGroup{}
			var imgFiles []string
			for _, image := range info.Images {
				wg.Add(1)
				image := image
				go func() {
					defer wg.Done()
					fname := path.Join("tmp", image.Path)
					if _, err := os.Stat(fname); err != nil {
						logrus.Infoln("exist", image.Path)

						return

					}

					resp, err2 := client.Get(image.Url)
					if err2 != nil {
						logrus.Warn(err2)
						return
					}
					//logrus.Infoln("download", image)
					file, err2 := os.OpenFile(fname, os.O_CREATE|os.O_RDWR, 0644)
					if err2 != nil {
						return
					}
					s, _ := filepath.Abs(fname)
					imgFiles = append(imgFiles, s)
					_, err := io.Copy(file, resp.Body)
					if err != nil {
						file.Close()
						os.Remove(fname)
						logrus.Warnln("download Failed", image)
						return
					}
					b := []byte("{\"retcode\":-5503007,\"retmsg\":\"download url has expired\",\"retryflag\":0}")
					buf := make([]byte, len(b))
					file.Read(buf)
					if bytes.Equal(b, buf) {
						file.Close()
						os.Remove(fname)
						logrus.Warnln("download Failed", image)
						return
					}
					file.Close()
					logrus.Infoln("download OK", image.Path)

				}()
			}
			wg.Wait()
			imgFiles = removeDuplicates(imgFiles)
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
			cmd.Start()
			cmd.Wait()
			os.Remove(imgFileListPath)
			ctx.UploadThisGroupFile(imgArchiveAbs, filepath.Base(imgArchiveAbs), "/")

			//cmd = exec.Command("7z", "a", "-y", "-p1145141919810", "-mhe", "twice_"+imgArchive, imgArchive)
			//cmd.Stdout = os.Stdout
			//cmd.Stderr = os.Stderr
			//cmd.Start()
			//cmd.Wait()
			//time.Sleep(3 * time.Second)
			//os.Remove(imgFileListPath)
			//os.Remove(fmt.Sprintf("%d.imgs.7z", id))

		})

	engine.OnMessage().Handle(func(ctx *zero.Ctx) {
		//println(string(ctx.Event.NativeMessage))
		var images = []File{}
		var videos = []File{}
		var links = []string{}
		var magnets = []string{}
		res := gjson.ParseBytes(ctx.Event.NativeMessage)
		for _, result := range res.Array() {
			msgType := result.Get("type").String()
			switch msgType {
			case "forward":
				//t := result.Get("json.type").String()
				Parse(result.Get("data.json"),
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
							var magnetRegexp, _ = regexp.Compile("([0-9a-zA-Z]{40})")
							allString = magnetRegexp.FindAllString(s, -1)
							for _, s2 := range allString {
								magnets = append(magnets, fmt.Sprintf("magnet:?xt=urn:btih:%s", s2))
							}
						} else if res.Get("type").String() == "ImageEntity" {
							println(res.Get("ImageUrl").String())
							images = append(images,
								File{
									Path: res.Get("FilePath").String(),
									Url:  res.Get("ImageUrl").String(),
								})
						} else {
							println(res.Get("ImageUrl").String())
							videos = append(videos,
								File{
									Path: res.Get("FilePath").String(),
									Url:  res.Get("VideoUrl").String(),
								})

						}
					})
			case "text":
				continue
			case "image":
				url := result.Get("data.url").String()
				err := downloadImageFromURL(url)
				if err != nil {
					logrus.Infoln(err.Error())
					continue
				}
			default:
				logrus.Infoln(msgType)
				continue
			}
		}
		images = removeDuplicatesFile(images)
		videos = removeDuplicatesFile(videos)
		links = removeDuplicates(links)
		magnets = removeDuplicates(magnets)
		info := ForwardInfo{
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

		if ctx.Event.GroupID == 564828920 || ctx.Event.GroupID == 924075421 || ctx.Event.GroupID == 946855395 {
			if len(info.Links)+len(info.Magnets)+len(info.Videos)+len(info.Images) != 0 {
				//ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(fmt.Sprintf("省流:%d条链接,%d条🧲,%d条视频,%d条图片", len(info.Links), len(info.Magnets), len(info.Videos), len(info.Images))))
			}
		}
		if len(info.Magnets) != 0 {
			ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("省流:🧲\n"+strings.Join(info.Magnets, "\n")))
		}
		// download
		if len(info.Links) == 0 {
			goto sendLinkEnd
		}
		{
			var base64ed []string
			md5Hash := md5.Sum([]byte(strings.Join(info.Links, "\n")))
			abs, _ := filepath.Abs(fmt.Sprintf("%x_links_txt", md5Hash))
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
		if len(info.Images) != 0 {
			client := http.Client{}
			var wg = sync.WaitGroup{}
			var imgFiles []string
			for _, image := range info.Images {
				wg.Add(1)
				image := image
				go func() {
					defer wg.Done()

					fname := path.Join("tmp", image.Path)
					if _, err := os.Stat(fname); err == nil {
						logrus.Infoln("exist", image)

						return

					}

					resp, err2 := client.Get(image.Url)
					if err2 != nil {
						logrus.Warn(err2)
						return
					}
					logrus.Infoln("download", image)
					file, err2 := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0644)
					if err2 != nil {
						return
					}
					s, _ := filepath.Abs(fname)
					imgFiles = append(imgFiles, s)
					io.Copy(file, resp.Body)
					_, err := io.Copy(file, resp.Body)
					if err != nil {
						file.Close()
						os.Remove(fname)
						logrus.Warnln("download Failed", image)
						return
					}
					file.Close()
					logrus.Infoln("download OK", image)

				}()
			}
			wg.Wait()
			imgFiles = removeDuplicates(imgFiles)
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
			cmd.Start()
			cmd.Wait()
			err = os.Remove(imgFileListPath)
			if err != nil {
				println(err.Error())
			}
			// upload
			if ctx.Event.GroupID == 564828920 || ctx.Event.GroupID == 924075421 || ctx.Event.GroupID == 946855395 {
				r := ctx.UploadThisGroupFile(imgArchiveAbs, fmt.Sprintf("img包()#%x.7z", md5Hash), "")
				if r.RetCode != 0 {
					logrus.Warn("returns", r.RetCode)
				}
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

func Parse(result gjson.Result, filter []string, callback func(res gjson.Result)) {
	t := result.Get("type").String()
	switch t {
	case "MultiMsgEntity":
		for _, r := range result.Get("Chains").Array() {
			Parse(r, filter, callback)
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
			Parse(r, filter, callback)
		}
		return
	case "XmlEntity":
		return
	}
	//logrus.Debugf("type: %s\n", t)

}
