package quote

import (
	"encoding/json"
	"fmt"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/alexflint/go-arg"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/extension/shell"
	"github.com/wdvxdr1123/ZeroBot/message"
	"html"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

type renderMessage struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
	Time  int64  `json:"time"`
	Qid   int64  `json:"qid"`
	Quote *struct {
		Name     string `json:"name"`
		Label    string `json:"label,omitempty"`
		Time     int64  `json:"time"`
		Qid      int64  `json:"qid"`
		Messages []struct {
			Type string      `json:"type"`
			Data interface{} `json:"data"`
		} `json:"messages"`
	} `json:"quote"`
	Messages []struct {
		Type string      `json:"type"`
		Data interface{} `json:"data"`
	} `json:"messages"`
}

func init() {
	engine := control.AutoRegister(&ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Brief:            "记录群友丢人时刻（",
		Help:             `- 回复消息的同时/q  ->渲染一张带有被回复人和话的大照片`,
	})
	client := &http.Client{}
	var quoteArgs struct {
		Size       int  `arg:"positional" default:"0" help:"不为零时渲染为历史消息记录"`
		GrayScale  bool `arg:"-g" default:"false" help:"灰度滤镜，默认关闭(仅Size为0时生效)"`   // 是否使用彩色
		Date       bool `arg:"-d" default:"false" help:"包含时间日期，默认关闭(仅Size为0时生效)"` // 是否使用彩色
		SingleUser bool `arg:"-s" default:"false" help:"仅查找被回复用户的消息"`
	}
	quoteArgsParser, _ := arg.NewParser(arg.Config{Program: "/q", IgnoreEnv: true}, &quoteArgs)
	engine.OnRegex(`^\[CQ:reply,id=(-?\d+)\].*/q\s*(.*)$`, zero.OnlyGroup).SetBlock(true).
		Handle(func(ctx *zero.Ctx) {
			err := quoteArgsParser.Parse(shell.Parse(ctx.State["regex_matched"].([]string)[2]))
			if err != nil {
				var buf = &strings.Builder{}
				buf.WriteString("参数似乎不对哦～" + err.Error() + "\n")
				quoteArgsParser.WriteHelp(buf)
				ctx.Send(buf.String())
				return
			}
			messageID := ctx.State["regex_matched"].([]string)[1]
			mid, err := strconv.Atoi(messageID)
			if err != nil {
				ctx.SendChain(message.Text("[ERROR]:", err))
				return
			}
			msg := ctx.GetMessage(mid)
			if quoteArgs.Size == 0 { //获取消息
				var name string
				if msg.Sender != nil {
					name = msg.Sender.Name()
				}

				avatar := fmt.Sprintf("http://q4.qlogo.cn/g?b=qq&nk=%d&s=640", msg.Sender.ID)
				content := BeautifyPlainText(ctx, msg.Elements, 0)
				date := ""
				if quoteArgs.Date {
					t := ctx.CallAction("get_msg", zero.Params{
						"message_id": messageID,
					}).Data
					date = time.UnixMilli(t.Get("time").Int() * 1000).Format("2006-01-02 15:04:05")
				}
				err, bytes := renderQuoteImage(client, name, avatar, content, date, &quoteArgs)
				if err != nil {
					ctx.SendChain(message.Text(err.Error()))
					return
				}
				ctx.SendChain(message.ImageBytes(bytes))
			} else {
				history := ctx.GetGroupMessageHistory(ctx.Event.GroupID, int64(mid))
				var body []*renderMessage
				array := history.Get("messages").Array()
				slices.Reverse(array)
				array = array[:quoteArgs.Size]
				slices.Reverse(array)
				for _, msg := range array {
					name := msg.Get("sender.card").String()
					if name == "" {
						name = msg.Get("sender.nickname").String()
					}
					label := ""
					if msg.Get("sender.role").String() != "member" {
						label = msg.Get("sender.role").String()
					}
					el := &renderMessage{
						Name:     name,
						Label:    label,
						Time:     msg.Get("time").Int(),
						Qid:      msg.Get("sender.user_id").Int(),
						Quote:    nil,
						Messages: nil,
					}

					if msg.Get("message[0].type").String() == "reply" {
						qm := ctx.GetMessage(msg.Get("message[0].data.id").Int())
						name := qm.Sender.Name()
						label := ""
						if qm.Sender.Role != "member" {
							label = qm.Sender.Role
						}
						var qMessages []struct {
							Type string      `json:"type"`
							Data interface{} `json:"data"`
						}
						for i, element := range qm.Elements {
							switch element.Type {
							case "replay":
							case "at":
								if i != 0 && qm.Elements[i-1].Type != "reply" {
									id, _ := strconv.Atoi(element.Data["qq"])
									minfo := ctx.GetGroupMemberInfo(ctx.Event.GroupID, int64(id), true)
									if minfo.Get("card").String() != "" {
										qMessages = append(qMessages, struct {
											Type string      `json:"type"`
											Data interface{} `json:"data"`
										}{Type: "at", Data: minfo.Get("card").String()})
									} else if minfo.Get("nickname").String() != "" {
										qMessages = append(qMessages, struct {
											Type string      `json:"type"`
											Data interface{} `json:"data"`
										}{Type: "at", Data: minfo.Get("nickname").String()})
									} else {
										qMessages = append(qMessages, struct {
											Type string      `json:"type"`
											Data interface{} `json:"data"`
										}{Type: "at", Data: element.Data["qq"]})
									}
								}
							case "text":
								qMessages = append(qMessages, struct {
									Type string      `json:"type"`
									Data interface{} `json:"data"`
								}{Type: "text", Data: element.Data["text"]})
							case "image":
								qMessages = append(qMessages, struct {
									Type string      `json:"type"`
									Data interface{} `json:"data"`
								}{Type: "image", Data: element.Data["url"]})
							}
						}
						el.Quote = &struct {
							Name     string `json:"name"`
							Label    string `json:"label,omitempty"`
							Time     int64  `json:"time"`
							Qid      int64  `json:"qid"`
							Messages []struct {
								Type string      `json:"type"`
								Data interface{} `json:"data"`
							} `json:"messages"`
						}{Name: name, Label: label, Time: strconv.IntSize, Qid: strconv.IntSize}
						el.Quote.Messages = qMessages

						var messages []struct {
							Type string      `json:"type"`
							Data interface{} `json:"data"`
						}
						for i, element := range msg.Get("message").Array() {
							switch element.Get("type").String() {
							case "replay":
							case "at":
								if i != 0 && msg.Get("message").Array()[i-1].Get("type").String() != "reply" {
									id := element.Get("data.qq").Int()
									minfo := ctx.GetGroupMemberInfo(ctx.Event.GroupID, int64(id), true)
									if minfo.Get("card").String() != "" {
										messages = append(messages, struct {
											Type string      `json:"type"`
											Data interface{} `json:"data"`
										}{Type: "at", Data: minfo.Get("card").String()})
									} else if minfo.Get("nickname").String() != "" {
										messages = append(messages, struct {
											Type string      `json:"type"`
											Data interface{} `json:"data"`
										}{Type: "at", Data: minfo.Get("nickname").String()})
									} else {
										messages = append(messages, struct {
											Type string      `json:"type"`
											Data interface{} `json:"data"`
										}{Type: "at", Data: id})
									}
								}
							case "text":
								messages = append(messages, struct {
									Type string      `json:"type"`
									Data interface{} `json:"data"`
								}{Type: "text", Data: element.Get("data.text").String()})
							case "image":
								messages = append(messages, struct {
									Type string      `json:"type"`
									Data interface{} `json:"data"`
								}{Type: "image", Data: element.Get("data.url").String()})
							}
						}
						el.Messages = messages
					} else {
						var messages []struct {
							Type string      `json:"type"`
							Data interface{} `json:"data"`
						}
						for i, element := range msg.Get("message").Array() {
							switch element.Get("type").String() {
							case "replay":
							case "at":
								if i != 0 && msg.Get("message").Array()[i-1].Get("type").String() != "reply" {
									id := element.Get("data.qq").Int()
									minfo := ctx.GetGroupMemberInfo(ctx.Event.GroupID, int64(id), true)
									if minfo.Get("card").String() != "" {
										messages = append(messages, struct {
											Type string      `json:"type"`
											Data interface{} `json:"data"`
										}{Type: "at", Data: minfo.Get("card").String()})
									} else if minfo.Get("nickname").String() != "" {
										messages = append(messages, struct {
											Type string      `json:"type"`
											Data interface{} `json:"data"`
										}{Type: "at", Data: minfo.Get("nickname").String()})
									} else {
										messages = append(messages, struct {
											Type string      `json:"type"`
											Data interface{} `json:"data"`
										}{Type: "at", Data: id})
									}
								}
							case "text":
								messages = append(messages, struct {
									Type string      `json:"type"`
									Data interface{} `json:"data"`
								}{Type: "text", Data: element.Get("data.text").String()})
							case "image":
								messages = append(messages, struct {
									Type string      `json:"type"`
									Data interface{} `json:"data"`
								}{Type: "image", Data: element.Get("data.url").String()})
							}
						}
						el.Messages = messages
					}
					body = append(body, el)
				}
				marshal, err := json.Marshal(body)
				if err != nil {
					ctx.SendChain(message.Text(err.Error()))
					return
				}
				err, bytes := renderHistoryImage(client, string(marshal), &quoteArgs)
				if err != nil {
					ctx.SendChain(message.Text(err.Error()))
					return
				}
				ctx.SendChain(message.ImageBytes(bytes))
			}
		})
}

func renderHistoryImage(client *http.Client, j string, args *struct {
	Size       int  `arg:"positional" default:"0" help:"不为零时渲染为历史消息记录"`
	GrayScale  bool `arg:"-g" default:"false" help:"灰度滤镜，默认关闭(仅Size为0时生效)"`
	Date       bool `arg:"-d" default:"false" help:"包含时间日期，默认关闭(仅Size为0时生效)"`
	SingleUser bool `arg:"-s" default:"false" help:"仅查找被回复用户的消息"`
}) (error, []byte) {
	postData := url.Values{}
	postData.Set("messages", j)
	p, _ := url.Parse("http://127.0.0.1:8888/message?dpi=F2X&fullPage&quality=90&fit-content=true")
	if !args.GrayScale {
		query := p.Query()
		query.Set("color", "true")
		p.RawQuery = query.Encode()
	}
	logrus.Debugf("[%s] rendering %s", "quote", p.String())
	response, err := client.Post(
		p.String(),
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

// BeautifyPlainText 美观输出消息
func BeautifyPlainText(ctx *zero.Ctx, m message.Message, indent int) string {
	sb := strings.Builder{}
	for _, element := range m {
		fmt.Println(indent, element.Type, element.Data)
	}
	for i := 0; i < len(m); i++ {
		val := m[i]
		switch val.Type {
		case "text":
			sb.WriteString(html.EscapeString(val.Data["text"]))
		case "image":
			//sb.WriteString("[图片]")
		case "at":
			qid, err := strconv.Atoi(val.Data["qq"])
			if err != nil {
				sb.WriteString("@" + val.Data["qq"])
			} else {
				info := ctx.GetGroupMemberInfo(ctx.Event.GroupID, int64(qid), true)
				if info.Get("card").String() != "" {
					sb.WriteString("@" + html.EscapeString(info.Get("card").String()))
				} else if info.Get("nickname").String() != "" {
					sb.WriteString("@" + html.EscapeString(info.Get("nickname").String()))
				}
			}
		case "reply":
			msg := ctx.GetMessage(val.Data["id"])
			fmt.Println(msg.Sender)
			if msg.Sender != nil {
				sb.WriteString(fmt.Sprintf(`<div class="quote"">%s</div>`, BeautifyPlainText(ctx, msg.Elements, indent+1)))
				if len(m) > i+1 && msg.Sender != nil && m[i+1].Type == "at" && m[i+1].Data["qq"] == strconv.FormatInt(msg.Sender.ID, 10) {
					i += 1
				}
			}
		}
	}
	return fmt.Sprintf(`<div class="qmsg">%s</div>`, sb.String())
}

func renderQuoteImage(client *http.Client, name string, avatar string, message string, date string, arg *struct {
	Size       int  `arg:"positional" default:"0" help:"不为零时渲染为历史消息记录"`
	GrayScale  bool `arg:"-g" default:"false" help:"灰度滤镜，默认关闭(仅Size为0时生效)"`
	Date       bool `arg:"-d" default:"false" help:"包含时间日期，默认关闭(仅Size为0时生效)"`
	SingleUser bool `arg:"-s" default:"false" help:"仅查找被回复用户的消息"`
}) (error, []byte) {
	postData := url.Values{}
	postData.Set("userName", name)
	postData.Set("userAvatar", avatar)
	postData.Set("message", message)
	postData.Set("date", date)
	logrus.Infof("rendering %v, %s", name, message)
	p, _ := url.Parse("http://127.0.0.1:8888/quote?dpi=F1_5X&scale=DEVICE&w=500&fullPage&quality=90")
	if !arg.GrayScale {
		query := p.Query()
		query.Set("color", "true")
		p.RawQuery = query.Encode()
	}
	println(p.String())
	response, err := client.Post(
		p.String(),
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
