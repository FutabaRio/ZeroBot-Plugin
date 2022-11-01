package weibo

import (
	"context"
	"fmt"
	ctrl "github.com/FloatTech/zbpctrl"
	"github.com/FloatTech/zbputils/control"
	"github.com/patrickmn/go-cache"
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

var ctxCel, cancel = context.WithCancel(context.Background())
var cacheMap = cache.New(5*time.Minute, 720*time.Hour)

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

func getWeiboMessageBox(url string) (string, string, []gjson.Result, string, string, string) {
	// 有个问题 需要排除置顶微博
	cont := getRequest(url)
	cards := gjson.Get(cont, "data.cards").Array()
	for _, card := range cards {
		isTop := gjson.Get(card.String(), "mblog.title").String()
		if isTop != "" {
			continue
		} else {
			profileId := gjson.Get(card.String(), "profile_type_id").String()
			msgText := gjson.Get(card.String(), "mblog.text").String()
			msgPic := gjson.Get(card.String(), "mblog.pics.#.url").Array()
			scheme := gjson.Get(card.String(), "scheme").String()
			username := gjson.Get(card.String(), "mblog.user.screen_name").String()
			createdAt, _ := time.Parse(time.RubyDate, gjson.Get(card.String(), "mblog.created_at").String())
			return profileId, msgText, msgPic, scheme, username, createdAt.String()
		}
	}
	return "", "", nil, "", "", ""
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
	fmt.Println("weibo插件加载")
	engine := control.Register("weiboMessage", &ctrl.Options[*zero.Ctx]{
		DisableOnDefault: false,
		Help:             "--help weibo Message",
	})
	engine.OnFullMatch("开启订阅").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		running(ctxCel, ctx)
	})
	engine.OnFullMatch("关闭订阅").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		stop(ctx)
	})
	engine.OnPrefix("订阅").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		args := ctx.State["args"].(string)
		getChannels(args)
		for _, item := range channelItemData {
			if item.ChannelKey == args {
				getWeiboLink(item.TestUrl, ctx)
			}
		}
	})
	engine.OnPrefix("取消订阅").SetBlock(true).Handle(func(ctx *zero.Ctx) {
		arg := ctx.State["args"].(string)
		delChannels(arg, ctx)
	})

}

func getWeiboLink(url string, ctx *zero.Ctx) {
	conn := getRequest(url)
	value := gjson.Get(conn, "data.userInfo.screen_name")
	ctx.Send(message.Message{
		message.Text("已经成功订阅  :" + value.String()),
	})
}
func getChannels(args ...string) {
	for _, item := range args {
		channelItemData = append(channelItemData, &channelItem{
			ChannelKey: item,
			TestUrl:    testApi + item,
			ContUri:    contApi + item,
		})
		fmt.Println("Get channels...", item)
	}
	return
}
func delChannels(arg string, ctx *zero.Ctx) {
	for i, item := range channelItemData {
		if item.ChannelKey == arg {
			channelItemData = append(channelItemData[:i], channelItemData[i+1:]...)
		}
	}
	ctx.Send(message.Message{
		message.Text("取消订阅", arg),
	})
}
func running(ctxCel context.Context, ctx *zero.Ctx) {
	for {
		ticker := time.NewTicker(60 * time.Second)
		select {
		case <-ticker.C:
			for _, item := range channelItemData {
				cUrl := item.ContUri
				pId, mText, mPic, scheme, username, creatAt := getWeiboMessageBox(cUrl)
				_, ok := cacheMap.Get(pId)
				if ok == false {
					cacheMap.Set(pId, true, cache.NoExpiration)
					ctx.Send(message.Message{
						message.Text(creatAt + "\n" + username + "发布了微博:\n" + TrimHtml(mText) + "\n\nURL:" + scheme),
					})
					for _, picUrl := range mPic {
						ctx.Send(message.Message{
							message.ImageBytes(getImageByUrl(picUrl.String())),
						})
					}
				} else {
					fmt.Println("命中缓存了，没有发布新的微博")
				}
			}
		case <-ctxCel.Done():
			break
		}
	}
}
func stop(ctx *zero.Ctx) {
	ctx.Send(message.Message{
		message.Text("关闭订阅成功"),
	})
	cacheMap.Flush()
	cancel()
}
