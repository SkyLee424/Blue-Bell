package utils

import (
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/spf13/viper"
)

var node *snowflake.Node

func InitSnowflake() {
	startTime := viper.GetString("server.start_time") // 读取配置文件
	machineID := viper.GetInt64("server.machine_id")

	st, err := time.Parse("2006-01-02", startTime)
	if err != nil {
		panic(err.Error())
	}

	snowflake.Epoch = st.UnixNano() / 1000000 // 设置起始时间戳
	node, err = snowflake.NewNode(machineID)  // 设置节点编号
	if err != nil {
		panic(err)
	}
}

func GenSnowflakeID() int64 {
	return node.Generate().Int64()
}
