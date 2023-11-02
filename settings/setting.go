package settings

import "github.com/spf13/viper"

func InitSettings(confPath string) {
	viper.SetDefault("server.ip", "")
	viper.SetDefault("server.port", 1145)
	viper.SetDefault("server.lang", "zh")
	viper.SetDefault("server.start_time", "2023-10-14")   // 项目开始时间
	viper.SetDefault("server.machine_id", 1)              // 节点默认编号
	viper.SetDefault("server.develop_mode", false)
	viper.SetDefault("server.shutdown_waitting_time", 30) // 收到 SIGINT 信号后，超过 30s，服务器将强制退出

	viper.SetDefault("mysql.driverName", "mysql")
	viper.SetDefault("mysql.host", "127.0.0.1")
	viper.SetDefault("mysql.port", 3306)
	viper.SetDefault("mysql.username", "root")
	viper.SetDefault("mysql.password", "123456")
	viper.SetDefault("mysql.database", "bluebell")
	viper.SetDefault("mysql.charset", "utf8mb4")

	viper.SetDefault("redis.host", "127.0.0.1")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.poolsize", 10)
	viper.SetDefault("redis.max_oper_time", 3)
	viper.SetDefault("redis.cache_key_tls", 60)
	viper.SetDefault("redis.hot_key_tls", 60)

	viper.SetDefault("logger.level", 0)
	viper.SetDefault("logger.path", "./logs/bluebell.log")
	viper.SetDefault("logger.max_size", 16)
	viper.SetDefault("logger.max_backups", 5)
	viper.SetDefault("logger.compress", false)
	viper.SetDefault("logger.console", true)

	viper.SetDefault("service.token.access_token_expire_duration", 86400)
	viper.SetDefault("service.token.refresh_token_expire_duration", 864000)

	viper.SetDefault("service.post.active_time", 604800)
	viper.SetDefault("service.post.persistence_interval", 43200)
	viper.SetDefault("service.post.content_max_length", 256)

	viper.SetDefault("service.swagger.enable", true)

	viper.SetConfigFile(confPath)

	if err := viper.ReadInConfig(); err != nil {
		panic(err.Error())
	}
}
