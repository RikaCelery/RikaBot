package spider

import (
	"github.com/FloatTech/ZeroBot-Plugin/utils"
	"github.com/sirupsen/logrus"
	"github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
)

func preprocess(ctx *zero.Ctx) bool {
	if len(ctx.Event.Message) == 0 {
		return false
	}
	forwards := make([]message.Segment, 0, 4)
	segments := make([]message.Segment, 0, len(ctx.Event.Message))
	for _, segment := range ctx.Event.Message {
		if segment.Type == "forward" {
			logrus.Debugf("[spider] extracting... %s", utils.Truncate(segment.CQCode(), 40))
			forwards = append(forwards, segment)
		} else {
			segments = append(segments, segment)
		}
	}
	//hasher := md5.New()
	for len(forwards) != 0 {
		R := utils.ParallelMap(forwards, 4, func(v message.Segment) (message.Message, error) {
			forwardMessage := ctx.GetForwardMessage(v.Data["id"])
			ctx.State["forward_id"] = v.Data["id"]
			logrus.Infof("[spider] size %s:%d", v.Data["id"], len(forwardMessage.Get("message.#.data.content|@flatten").Array()))
			return message.ParseMessageFromArray(forwardMessage.Get("message.#.data.content|@flatten")), nil
		})
		forwards = make([]message.Segment, 0, 4)
		for _, m := range R {
			if m.Err != nil {
				logrus.Errorf("[spider] download forward error: %s", m.Err)
				continue
			}
			for _, segment := range *m.Ret {
				if segment.Type == "forward" {
					logrus.Debugf("[spider] extracting... %s", utils.Truncate(segment.CQCode(), 40))
					forwards = append(forwards, segment)
				} else {
					segments = append(segments, segment)
				}
			}
		}

		for i := 0; i < len(segments); i++ {
			segment := segments[i]
			switch segment.Type {
			case "forward":
				forwards = append(forwards, segment)
			}
		}
	}
	R := utils.ParallelMap(segments, 64, func(v message.Segment) (message.Segment, error) {
		var filename = make(chan string, 1)
		switch v.Type {
		case "video":
			URL := v.Data["url"]
			err := downloadVideoFromURL(URL, filename)
			if err != nil {
				logrus.Warning("download video failed: ", err.Error())
				v.Data["hash"] = "[image_error]"
				return v, nil
			}
			v.Data["local_filename"] = <-filename
		case "image":
			URL := v.Data["url"]
			err := downloadImageFromURL(URL, filename)
			if err != nil {
				logrus.Warning("download image(%s) failed: ", URL, err.Error())
				v.Data["hash"] = "[image_error]"
				return v, nil
			}
			v.Data["local_filename"] = <-filename
			phash, md5hash, err := getImageHashFromFile(v.Data["local_filename"])
			if err != nil {
				logrus.Warningf("get image hash failed: %s, %v", v.Data["local_filename"], err.Error())
				return v, nil
			}
			v.Data["phash"] = phash
			v.Data["md5"] = md5hash
		}
		return v, nil
	})
	ctx.State["DATA"] = R
	return true
}
