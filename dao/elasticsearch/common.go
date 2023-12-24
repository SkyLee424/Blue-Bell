package elasticsearch

import (
	"fmt"
	"math"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/spf13/viper"
)

var clnt *elasticsearch.TypedClient
var lowlevelClnt *elasticsearch.Client

var postActiveDay int64

func Init() {
	var err error
	host := viper.Get("elasticsearch.host")
	port := viper.GetInt("elasticsearch.port")
	dsn := fmt.Sprintf("http://%v:%v", host, port)
	cfg := elasticsearch.Config{
		Addresses: []string{dsn}, // 读取配置文件
	}

	// 创建客户端
	clnt, err = elasticsearch.NewTypedClient(cfg)
	if err != nil {
		panic(err.Error())
	}
	lowlevelClnt, err = elasticsearch.NewClient(cfg)
	if err != nil {
		panic(err.Error())
	}

	// 检查索引有没有创建
	resp, err := lowlevelClnt.Indices.Get([]string{"bluebell_post_index"})
	if err != nil {
		panic(err.Error())
	}
	if resp.StatusCode == 404 {
		panic("elasticsearch: bluebell_post_index has not been created yet")
	}

	// 读取配置
	activeSecond := viper.GetInt64("service.post.active_time")
	postActiveDay = int64(math.Ceil(float64(activeSecond) / 86400.0))
}

func GetClnt() *elasticsearch.TypedClient {
	return clnt
}

func GetLowLevelClnt() *elasticsearch.Client {
	return lowlevelClnt
}
