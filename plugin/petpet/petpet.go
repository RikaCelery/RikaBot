// Package petpet 外置petpet插件
package petpet

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/ttl"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/golang/freetype/truetype"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/FloatTech/ZeroBot-Plugin/utils"
)

type petType struct {
	To   bool
	From bool
}

var (
	keywords = map[string]string{}
	petTypes = map[string]petType{}
)

func getData(url string) (data []byte, err error) {
	split := strings.Split(url, "/")
	cache := "data/petpet/" + split[len(split)-1]
	if utils.Exists(cache) {
		return os.ReadFile(cache)
	}
	var response *http.Response
	response, err = http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		s := fmt.Sprintf("status code: %d", response.StatusCode)
		err = errors.New(s)
		return
	}
	data, err = io.ReadAll(response.Body)
	_ = utils.WriteBytes(cache, data)
	return
}

// drawText 在图像上绘制文本
func drawText(img *image.RGBA, face font.Face, text string) {
	height := 18
	{
		offset := 1
		d := &font.Drawer{
			Dst:  img,
			Src:  image.White,
			Face: face,
		}

		// 分割文本为多行
		lines := strings.Split(text, "\n")

		// 逐行绘制文本
		for i, line := range lines {
			d.Dot = fixed.P(3, height*(i+1)+offset)
			d.DrawString(line)
			d.Dot = fixed.P(3+offset, height*(i+1))
			d.DrawString(line)
			d.Dot = fixed.P(3, height*(i+1)+offset)
			d.DrawString(line)
			d.Dot = fixed.P(3-offset, height*(i+1))
			d.DrawString(line)
			d.Dot = fixed.P(3+offset, height*(i+1)+offset)
			d.DrawString(line)
			d.Dot = fixed.P(3+offset, height*(i+1)-offset)
			d.DrawString(line)
			d.Dot = fixed.P(3-offset, height*(i+1)+offset)
			d.DrawString(line)
			d.Dot = fixed.P(3-offset, height*(i+1)-offset)
			d.DrawString(line)
		}
	}
	{
		d := &font.Drawer{
			Dst:  img,
			Src:  image.Black,
			Face: face,
		}
		d.Dot = fixed.P(3, height) // 设置文本绘制的起始位置

		// 分割文本为多行
		lines := strings.Split(text, "\n")

		// 逐行绘制文本
		for i, line := range lines {
			d.DrawString(line)
			d.Dot = fixed.P(3, height*(i+2))
		}
	}
}

// download every gif(first frame), resize to 100x100, and paste into a big gif
func renderPreview(links []gjson.Result) ([]*image.RGBA, error) {
	const (
		frameWidth  = 170
		frameHeight = frameWidth
		ImageWidth  = 10 * frameWidth
	)

	var (
		imgs               []*image.RGBA
		currentImg         *image.RGBA
		currentX, currentY int
		imgIndex           int
	)
	maxHeight := int(math.Ceil(float64(len(links))/10)) * frameHeight
	// 初始化第一个大图像
	currentImg = image.NewRGBA(image.Rect(0, 0, ImageWidth, maxHeight))
	imgs = append(imgs, currentImg)

	// 加载自定义字体
	fontBytes, err := os.ReadFile("data/Font/GlowSansSC-Normal-ExtraBold.ttf")
	if err != nil {
		return nil, err
	}
	parsedFont, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	face := truetype.NewFace(parsedFont, &truetype.Options{
		Size: 18, // 字体大小
		DPI:  72, // 每英寸点数
	})
	for _, res := range links {
		var resizedFrame *image.RGBA

		// 下载GIF
		key := res.Get("key").Str
		resp, err := getData(fmt.Sprintf("http://127.0.0.1:2333/preview/%s.gif", key))
		if err == nil {
			// 解码GIF
			gifImg, err := gif.DecodeAll(bytes.NewReader(resp))
			if err != nil {
				log.Warningf("failed to decode gif: %v %s", err.Error(), key)
				// return nil, err
				continue
			}
			// 获取第一帧
			firstFrame := gifImg.Image[0] // 缩放第一帧为100x100
			resizedFrame = resizeImage(firstFrame, frameWidth, frameHeight)

			goto normal
		}
		// 下载png
		resp, err = getData(fmt.Sprintf("http://127.0.0.1:2333/preview/%s.png", key))
		if err == nil {
			// 解码png
			pngImg, err := png.Decode(bytes.NewReader(resp))
			if err != nil {
				log.Warningf("failed to decode png: %v %s", err.Error(), key)
				// return nil, err
				continue
			}
			resizedFrame = resizeImage(pngImg, frameWidth, frameHeight)
			goto normal
		}
		log.Warningf("failed to download preview: %v %s", err.Error(), key)
		// return nil, err
		continue
	normal:

		names := []string{key}
		for _, result := range res.Get("alias").Array() {
			names = append(names, result.Str)
		}
		// 在缩放后的第一帧上绘制文本
		drawText(resizedFrame, face, strings.Join(names, "\n"))

		// 计算位置
		if currentX+frameWidth > currentImg.Bounds().Dx() {
			currentX = 0
			currentY += frameHeight
		}

		// 如果当前大图高度超过1000px，创建新的大图
		if currentY+frameHeight > maxHeight {
			currentImg = image.NewRGBA(image.Rect(0, 0, ImageWidth, maxHeight))
			imgs = append(imgs, currentImg)
			currentY = 0
			imgIndex++
		}

		// 将缩放后的第一帧绘制到大图像上
		draw.Draw(currentImg, image.Rect(currentX, currentY, currentX+frameWidth, currentY+frameHeight), resizedFrame, image.Point{}, draw.Src)

		// 更新当前位置
		currentX += frameWidth
	}

	return imgs, nil
}

// resizeImage 将图像缩放到指定的宽度和高度，并保持纵横比，使用 contain 方式
func resizeImage(src image.Image, targetWidth, targetHeight int) *image.RGBA {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	// 计算目标尺寸，保持纵横比，使用 contain 方式
	scaleWidth := float64(targetWidth) / float64(srcWidth)
	scaleHeight := float64(targetHeight) / float64(srcHeight)
	scale := scaleWidth
	if scaleHeight < scaleWidth {
		scale = scaleHeight
	}

	width := int(float64(srcWidth) * scale)
	height := int(float64(srcHeight) * scale)

	// 创建目标图像
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// 缩放图像
	draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	// 创建最终的目标图像
	finalDst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	// 计算目标图像的中心位置
	offsetX := (targetWidth - width) / 2
	offsetY := (targetHeight - height) / 2

	// 将缩放后的图像绘制到目标图像的中心位置
	draw.Draw(finalDst, finalDst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(finalDst, dst.Bounds().Add(image.Pt(offsetX, offsetY)), dst, image.Point{}, draw.Over)

	return finalDst
}
func getPetPreview() ([]*image.RGBA, error) {
	data, err := web.GetData("http://127.0.0.1:2333/petpet")
	if err != nil {
		return nil, err
	}
	result := gjson.ParseBytes(data)
	for _, res := range result.Get("petData").Array() {
		keywords[res.Get("key").Str] = res.Get("key").Str
		pettype := petType{}
		for _, r := range res.Get("types").Array() {
			switch r.Str {
			case "TO":
				pettype.To = true
			case "FROM":
				pettype.From = true
			}
			petTypes[res.Get("key").Str] = pettype
		}
		for _, r := range res.Get("alias").Array() {
			keywords[r.Str] = res.Get("key").Str
		}
	}
	previewGif, err := renderPreview(result.Get("petData").Array())
	if err != nil {
		return nil, err
	}
	return previewGif, nil
}
func init() { // 插件主体
	cache := ttl.NewCache[string, []*image.RGBA](24 * time.Hour)
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault:  false,
		Brief:             "摸摸头",
		Help:              "生成各种各样奇奇怪怪的图片\n- pet 显示所有可以生成的动图\n- )+关键词 [可选的一些数据比如文字或者@其他人]   制作一个动图",
		PrivateDataFolder: "petpet",
	}).ApplySingle(ctxext.NoHintSingle)
	go func() {
		preview, err := getPetPreview()
		if err != nil {
			panic(err)
		}
		log.Infof("[petpet] 预览缓存完成: %d", len(preview))
		cache.Set("big", preview)
	}()
	engine.OnFullMatch("pet").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		previewGif := cache.Get("big")
		if previewGif == nil {
			var err error
			previewGif, err = getPetPreview()
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			cache.Set("big", previewGif)
		}
		for _, preview := range previewGif {
			buf := &bytes.Buffer{}
			err := jpeg.Encode(buf, preview, &jpeg.Options{Quality: 90})
			if err != nil {
				ctx.SendChain(message.Text("ERROR: ", err))
				return
			}
			ctx.Send(message.ImageBytes(buf.Bytes()))
		}
	})
	engine.OnRegex(`^[）)(（] ?(\S+)`, func(ctx *zero.Ctx) bool {
		matched := ctx.State["regex_matched"].([]string)
		key := matched[1]
		for s, v := range keywords {
			if strings.HasPrefix(key, s) {
				ctx.State["petpet_key"] = v
				ctx.State["petpet_keyword"] = s
				return true
			}
		}
		return false
	}).SetBlock(true).Handle(func(ctx *zero.Ctx) {
		key := ctx.State["petpet_key"].(string)
		keyword := ctx.State["petpet_keyword"].(string)
		var res = struct {
			Key  string `json:"key,omitempty"`
			From struct {
				Name   string `json:"name,omitempty"`
				Avatar string `json:"avatar,omitempty"`
			} `json:"from,omitempty"`
			To struct {
				Name   string `json:"name,omitempty"`
				Avatar string `json:"avatar,omitempty"`
			} `json:"to,omitempty"`
			Group struct {
				Name   string `json:"name,omitempty"`
				Avatar string `json:"avatar,omitempty"`
			} `json:"group,omitempty"`
			Bot struct {
				Name   string `json:"name,omitempty"`
				Avatar string `json:"avatar,omitempty"`
			} `json:"bot,omitempty"`
			RandomAvatarList []string `json:"randomAvatarList,omitempty"`
			TextList         []string `json:"textList,omitempty"`
		}{
			Key: key,
			To: struct {
				Name   string `json:"name,omitempty"`
				Avatar string `json:"avatar,omitempty"`
			}{Name: ctx.Event.Sender.Name(), Avatar: fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", ctx.Event.UserID)},
			From: struct {
				Name   string `json:"name,omitempty"`
				Avatar string `json:"avatar,omitempty"`
			}{Name: ctx.Event.Sender.Name(), Avatar: fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", ctx.Event.UserID)},
			Group: struct {
				Name   string `json:"name,omitempty"`
				Avatar string `json:"avatar,omitempty"`
			}{Name: ctx.Event.Sender.Name(), Avatar: fmt.Sprintf("https://p.qlogo.cn/gh/%d/640", ctx.Event.GroupID)},
			Bot: struct {
				Name   string `json:"name,omitempty"`
				Avatar string `json:"avatar,omitempty"`
			}{Name: ctx.Event.Sender.Name(), Avatar: fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", ctx.Event.SelfID)},
			RandomAvatarList: make([]string, 0, 100),
			TextList:         make([]string, 0, 4),
		}
		// for _, result := range ctx.GetThisGroupMemberList().Array() {
		//	res.RandomAvatarList = append(res.RandomAvatarList, fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", result.Int()))
		//}
		split := strings.Split(ctx.Event.Message[0].Data["text"], keyword)
		for _, word := range strings.Split(strings.TrimSpace(split[len(split)-1]), " ") {
			if s := strings.TrimSpace(word); s != "" {
				res.TextList = append(res.TextList, s)
			}
		}
		pettype := petTypes[key]
		ats := make([]int64, 0, 5)
		for _, msg := range ctx.Event.Message[1:] {
			switch msg.Type {
			case "at":
				qq, _ := strconv.ParseInt(msg.Data["qq"], 10, 64)
				ats = append(ats, qq)
			case "text":
				for _, word := range strings.Split(strings.TrimSpace(msg.String()), " ") {
					if s := strings.TrimSpace(word); s != "" {
						res.TextList = append(res.TextList, s)
					}
				}
			}
		}
		if len(ats) == 1 && (pettype.From && !pettype.To || pettype.To && !pettype.From) {
			if pettype.From {
				res.From.Name = ctx.CardOrNickName(ats[0])
				res.From.Avatar = fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", ats[0])
			}
			if pettype.To {
				res.To.Name = ctx.CardOrNickName(ats[0])
				res.To.Avatar = fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", ats[0])
			}
		}
		if len(ats) == 1 && pettype.From && pettype.To {
			res.To.Name = ctx.CardOrNickName(ats[0])
			res.To.Avatar = fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", ats[0])
		}
		if len(ats) >= 2 && pettype.From && pettype.To {
			idx := 0
			for _, at := range ats {
				if idx == 0 {
					res.From.Name = ctx.CardOrNickName(at)
					res.From.Avatar = fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", at)
				}
				if idx == 1 {
					res.To.Name = ctx.CardOrNickName(at)
					res.To.Avatar = fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=640", at)
				}
				idx++
			}
		}
		fmt.Println(ats, pettype)
		data, err := web.PostData("http://127.0.0.1:2333/petpet", "application/json", strings.NewReader(utils.ToJSON(res)))
		if err != nil {
			ctx.SendChain(message.Text("ERROR: ", err))
			return
		}
		ctx.SendChain(message.ImageBytes(data))
	})
}
