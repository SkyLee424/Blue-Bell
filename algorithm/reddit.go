package algorithm

import (
	"math"
	"time"

	"github.com/spf13/viper"
)

// 基于 Reddit 投票算法，计算出一个帖子的分数
func GetPostScoreByReddit(timestamp, voteDiff int64) float64 {
	startTime := viper.GetString("server.start_time") // 读取配置文件
	st, _ := time.Parse("2006-01-02", startTime)

	t := timestamp - st.Unix() // 帖子新旧程度
	x := voteDiff              // 赞成票 - 反对票

	var y int8 // 投票方向
	if x > 0 {
		y = 1
	} else if x == 0 {
		y = 0
	} else {
		y = -1
	}

	z := math.Abs(float64(x)) // 肯定程度
	if z == 0 {
		z = 1
	}

	score := math.Log10(z) + float64(y)*float64(t)/45000.0
	return score
}
