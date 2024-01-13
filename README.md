# Blue-Bell

Blue-Bell 是一个使用 Go 语言构建的网络论坛式 Web 项目，目标是提供一个让用户进行交流、分享想法的平台。项目以高性能为核心目标。

## 使用到的中间件

Blue-Bell 使用了以下中间件：

- **MySQL**：作为主要的数据存储，用于存储用户信息、帖子、评论等数据。
- **Redis**：用于存储临时数据和缓存，如用户登录信息（token）、评论 ID 索引等。
- **ElasticSearch**：帖子的全文搜索功能基于此实现。
- **Kafka**：保证高并发写操作的可用性，「削峰」处理
- **Bleve**：一个基于 Go 的全文搜索库。
- **Ants**：一个高性能的 Go 语言协程池，用于协程复用。
- **SingleFlight**：防止缓存击穿。
- **GCache**：提供本地缓存支持。

## 实现的功能

Blue-Bell 实现了包括但不限于以下功能：

- **用户模块**：用户可以通过注册功能创建新的账户并完成登录。
- **帖子模块**：用户可以创建新的帖子，查看帖子详情，以及浏览帖子列表。
- **评论模块**：用户可以对帖子进行评论，查看评论，并有权删除他们自己的评论。
- **点赞模块**：用户可以给评论点赞或踩。

## 搭建 Blue-Bell 项目

这里使用 Docker 快速搭建 Blue-Bell 项目

### 配置文件

首先在项目根目录创建一个 container 文件夹，并创建 config 文件

```bash
mkdir container

cd container

mkdir config

mkdir kafka_data

touch config/config.json
```

config.json 的内容在 [这里](#配置说明)

### 制作 Blue-Bell Server 镜像

```Dockerfile
FROM golang:alpine AS builder

# 为我们的镜像设置必要的环境变量
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# 移动到工作目录：/build
WORKDIR /build

# 复制项目中的 go.mod 和 go.sum文件并下载依赖信息
COPY go.mod .
COPY go.sum .
RUN go mod download

# 将代码复制到容器中
COPY . .

# 将我们的代码编译成二进制可执行文件
RUN go build -o bluebell .

# 声明服务端口
EXPOSE 1145



# 创建一个小的镜像 #
FROM debian:stretch-slim

# 从builder镜像中把脚本拷贝到当前目录
COPY ./wait-for.sh /

# 拷贝配置文件
# COPY ./config/config.json /

COPY --from=builder /build/bluebell /

# 使用阿里源，将本地的 sources.list 文件复制到容器内的 /etc/apt/ 目录下
COPY sources.list /etc/apt/sources.list

RUN set -eux; \
	apt-get update; \
	apt-get install -y --allow-unauthenticated \
		--no-install-recommends \
		netcat; \
        chmod 755 wait-for.sh
```

执行：

```bash
docker build -t bluebell .
```

Blue-Bell Server 的 docker 镜像就创建好了

### 运行 Blue-Bell 项目

使用 Docker 快速搭建运行环境

这里的 mysql、redis、kafka 都是单节点，并且没有使用 es 作为搜索引擎

docker-compose 文件内容如下：

```yml
# yaml 配置
version: "3.7"
services:
  mysql8:
    image: "mysql:latest"
    ports:
      - "13306:3306"
    command: "--default-authentication-plugin=mysql_native_password --init-file /data/application/init.sql"
    environment:
      MYSQL_ROOT_PASSWORD: "123456"
      MYSQL_DATABASE: "bluebell"
      MYSQL_PASSWORD: "123456"
    volumes:
      - ./init.sql:/data/application/init.sql
  redis5:
    image: "redis:latest"
    ports:
      - "16379:6379"
    environment:
      REDIS_PASSWORD: "123456"
  # elasticsearch8:
  #   image: "elasticsearch:8.10.3"
  #   environment:
  #     - node.name=elasticsearch
  #     - ES_JAVA_OPTS=-Xms512m -Xmx512m
  #     - discovery.type=single-node
  #     - xpack.security.enabled=false
  #   ports:
  #     - 9200:9200
  zookeeper:
    image: zookeeper
    container_name: zookeeper-1
    ports:
      - 12181:2181

  kafka-4:
    image: bitnami/kafka
    container_name: kafka-4
    ports:
      - "19093:9093"
    depends_on:
      - zookeeper
    environment:
      KAFKA_BROKER_ID: 0
      KAFKA_NUM_PARTITIONS: 3
      KAFKA_DEFAULT_REPLICATION_FACTOR: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper-1:2181
      KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka-4:9093
    volumes:
      - ./container/kafka_data:/kafka
      - /var/run/docker.sock:/var/run/docker.sock
  bluebell:
    image: bluebell:latest
    command: sh -c "./wait-for.sh mysql8:3306 redis5:6379 zookeeper:2181 kafka-4:9093 -- ./bluebell -c /data/application/config.json"
    depends_on:
      - mysql8
      - redis5
      # - elasticsearch8
      - kafka-4
    ports:
      - "1145:1145"
    volumes:
      - ./container/config:/data/application # 将本地 ./container/config 目录挂载到容器的 /data/application 目录下
      - ./container/logs:/logs # 映射容器内日志路径到本地的 ./container/log 目录
      - ./container/bluebell_post.bleve:/bluebell_post.bleve
      - /var/run/docker.sock:/var/run/docker.sock
```

执行 docker-compose 命令：

```bash
docker-compose up
```

部署完毕

**补充**：如果要使用 ES 作为搜索引擎，需要提前创建索引，索引的定义如下：

```json
// 创建索引
PUT /test_bluebell_post_v1
{
  "mappings": {
    "properties": {
      "post_id": {
        "type": "double",
        "index": false
      },
      "title": {
        "type": "text",
        "analyzer": "ik_max_word"
      },
      "content": {
        "type": "text",
        "analyzer": "ik_smart"
      },
      "created_time": {
        "type": "date", 
        "format": "yyyy-MM-dd HH:mm:ss"
      }
    }
  }
}

// 创建索引别名
POST /_aliases
{
  "actions": [
    {
      "add": {
        "index": "test_bluebell_post_v1",
        "alias": "bluebell_post_index"
      }
    }
  ]
}
```

## 配置说明

```json
{
    "server": {
        "ip": "",
        "port": 1145, // Blue-Bell 的端口号
        "lang": "zh",
        "start_time": "2023-10-14",  // 项目起始运行时间，被用于生成 snowflake ID
        "machine_id": 1,             // 节点号，被用于生成 snowflake ID
        "develop_mode": true,        // 是否为开发模式（控制日志输出）
        "shutdown_waitting_time": 30 // 按下 control^c 后，超过该时间，强制关闭 server
    },
    "router": {
        "corf": {
            "frontend_path": "http://localhost:5173" // 前端的 url
        },
        "ratelimit":{
            "enable": true,  // 是否启用限流
            "rate": 3500,    // 平均每秒最大并发量
            "capacity": 5000 // 瞬时每秒最大并发量
        }
    },
    "mysql": {
        "driverName": "mysql", // 使用的驱动，建议不要更改，其它 db 没有测试过
        "host": "mysql8",      // db 的 host
        "port": 3306,          
        "username": "root",    
        "password": "123456",
        "database": "bluebell",
        "charset": "utf8mb4",
        "debug": false         // 是否开启 debug（开启后会打印所有执行的 SQL 语句到 terminal）
    },
    "redis":{
        "host": "redis5",
        "port": 6379,
        "password": "123456",
        "db": 0,
        "poolsize": 10,        // 连接池的最大连接数
        "max_oper_time": 3,    // 单次操作允许的最大时间
        "cache_key_tls": 60,   
        "hot_key_tls": 60
    },
    "elasticsearch":{
        "host": "elasticsearch8",
        "port": 9200,
        "enable": false
    },
    "bleve":{
        "enable": true
    },
    "kafka":{
        "addr":["kafka-4:9093"],
        "partition": {
            "comment": 6,
            "like": 6
        },
        "replication_factor": {
            "comment": 1,
            "like": 1
        },
        "retry":{           // 失败后的重试次数
            "producer": 5,
            "consumer": 5
        }
    },
    "localcache":{
        "size": 1024       // 本地缓存的大小（目前采取 LRU 淘汰策略）
    },
    "logger":{        
        "level": 0,                     // 日志级别
        "path": "./logs/bluebell.log",  // 日志输出路径
        "max_size": 16,                 // 单个日志文件的最大大小（KB）
        "max_backups": 5,               // 最多保存的日志文件个数，超出后删除最早的日志
        "compress": false,              // 是否压缩
        "console": true                 // 是否打印到 terminal
    },
    "service":{
        "token":{
            "access_token_expire_duration": 864000, // access_token 的过期时间（s）
            "refresh_token_expire_duration": 864000
        },
        "post":{
            "active_time": 604800,          // 帖子的活跃时间，超出该时间，首页不会展示该帖子
            "persistence_interval": 300,    // 每 persistence_interval 秒后检测过期的帖子
            "content_max_length": 256       // 帖子列表中，返回的单个帖子的内容最大长度（前端展示部分内容给用户预览）
        },
        "comment":{
            "index": {
                "remove_interval": 60,      // 每 remove_interval 秒检测一次
                "expire_time": 120          // 控制 commentID 索引缓存的过期时间
            },
            "content": {
                "remove_interval": 60,
                "expire_time": 90          // 控制评论内容缓存的过期时间
            },
            "count": {
                "persistence_interval": 90,
                "expire_time": 150         // 控制评论点赞数的过期时间
            },
            "like_hate_user": {
                "persistence_interval": 60,
                "remove_interval": 30,
                "like_expire_time": 30,    // 控制用户点赞过的评论 ID 缓存过期时间
                "hate_expire_time": 30
            }
        },
        "hot_post_list":{
            "refresh_time": 15, // 热帖排行榜的刷新时间
            "size": 5           // 排行榜有多少个帖子
        },
        "hot_spot":{
            "refresh_time": 1,    // 热点检测间隔
            "time_interval": 60,  // 基于 time_interval 秒内的数据来判断热点
            "size_for_post": 256, // 帖子的最大热点数
            "size_for_comment": 1024
        },
        "swagger":{
            "enable": true // 是否启用接口文档 API
        },
        "timeout": 3, // 单次请求允许的最长时间
        "rps": 10     // 下游的 rps
    }
}
```

----

该项目目前还是一个 demo，还有一些功能没实现，如用户模块仅支持注册和登录功能

后续~~也许~~会补充