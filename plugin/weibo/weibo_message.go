package weibo

import (
	"fmt"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type channelItem struct {
	ChannelKey string `json:"bytes"`
	TestUrl    string `json:"testApi"`
	ContUri    string `json:"contApi"`
}

var (
	testApi         = "https://m.weibo.cn/api/container/getIndex?containerid=100505"
	contApi         = "https://m.weibo.cn/api/container/getIndex?containerid=107603"
	channelItemData []*channelItem
)

func TrimHtml(src string) string {
	//将HTML标签全转换成小写
	re, _ := regexp.Compile("<[\\S\\s]+?>")
	src = re.ReplaceAllStringFunc(src, strings.ToLower)
	//去除STYLE
	re, _ = regexp.Compile("<style[\\S\\s]+?</style>")
	src = re.ReplaceAllString(src, "")
	//去除SCRIPT
	re, _ = regexp.Compile("<script[\\S\\s]+?</script>")
	src = re.ReplaceAllString(src, "")
	//去除所有尖括号内的HTML代码，并换成换行符
	re, _ = regexp.Compile("<[\\S\\s]+?>")
	src = re.ReplaceAllString(src, "\n")
	//去除连续的换行符
	re, _ = regexp.Compile("\\s{2,}")
	src = re.ReplaceAllString(src, "\n")
	return strings.TrimSpace(src)
}

func getRequest(url string) string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)
	result, _ := io.ReadAll(resp.Body)
	return string(result)
}

func getWeiboMessageBox(url string) (string, string, []gjson.Result) {
	cont := getRequest(url)
	profileId := gjson.Get(cont, "data.cards.0.profile_type_id").String()
	msgText := gjson.Get(cont, "data.cards.0.mblog.text").String()
	msgPic := gjson.Get(cont, "data.cards.0.mblog.pics.#.url").Array()
	return profileId, msgText, msgPic
}

func getImageByUrl(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
	}
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	return body
}

func init() {
	engine := control.Register("weiboMessage", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Help:             "--help weibo Message",
	})
	engine.OnPrefix("订阅").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		args := ctx.State["args"].(string)
		ch := getChannels(args)
		for _, item := range ch {
			linkMsg := getWeiboLink(item.TestUrl)
			ctx.Send(message.Message{
				message.Text(linkMsg),
			})
		}
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			var tmpId string
			for _, item := range ch {
				profileId, msgText, msgPic := getWeiboMessageBox(item.ContUri)
				if tmpId != profileId {
					ctx.Send(message.Message{
						message.Text(":\n" + TrimHtml(msgText)),
					})
					for _, picUrl := range msgPic {
						ctx.Send(message.Message{
							message.ImageBytes(getImageByUrl(picUrl.String())),
						})
					}
					tmpId = profileId
				}
			}
		}
	})
}

func getWeiboLink(url string) string {
	conn := getRequest(url)
	value := gjson.Get(conn, "data.userInfo.screen_name")
	return "已经成功订阅  :" + value.String()
}

func getChannels(args ...string) []*channelItem {
	for _, item := range args {
		channelItemData = append(channelItemData, &channelItem{
			ChannelKey: item,
			TestUrl:    testApi + item,
			ContUri:    contApi + item,
		})
	}
	return channelItemData
}

func delChannels(arg string) []*channelItem {
	r := channelItemData
	for i, item := range r {
		if item.ChannelKey == arg {
			r = append(r[:i], r[i+1:]...)
		}
	}
	return r
}

func running() {
	r := channelItemData
	ticker := time.NewTicker(60 * time.Second)
	for range ticker.C {
		for _, item := range r {
			linkMsg := getWeiboLink(item.TestUrl)
			fmt.Println(linkMsg)
		}
	}
}

// Send 快捷发送
