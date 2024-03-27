# Blue-Bell

2024.3.27 更新：

主要是性能优化：

**帖子模块**

- 重写获取帖子列表逻辑，相关 API 的 QPS 提升近 2 倍
- 自动创建索引：post.idx_author_id

**评论模块**

使用 lua 脚本处理点赞逻辑，减少锁时间，大幅提高点赞 API 的 QPS（提升近 5 倍）

2024.2.18 更新：

**用户模块**

- 注册时引入了邮箱验证
- 修改用户信息（上传用户头像）

**帖子模块**

- 在个人中心展示帖子列表
- 帖子删除
- 上传图片

**评论模块**

- 展示用户头像

----

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
            "like": 6,
            "email": 2
        },
        "replication_factor": {
            "comment": 1,
            "like": 1,
            "email": 1
        },
        "retry":{           // 失败后的重试次数
            "producer": 5,
            "consumer": 5
        }
    },
    "qiniu":{
        "access_key": "",       // 七牛云的 AK
        "secret_key": "",       // 七牛云的 SK
        "scope": "blue-bell",   // 对象空间名称
        "expires": 60,          // 生成的 update_token 的过期时间（s）
        "base_url": "",         // 七牛云对象空间的基础 url，例如："http://images.skylee.top/"
        "callback_base_url": "" // 七牛云回调请求的基础 url，格式为："http://前端 ip:前端 port/"
    },
    "email":{           // 邮件使用 SMTP 协议
        "username": "", // 发送邮箱
        "password": "", // token
        "host": "",     // 发送邮件服务器 host
        "port": 587,    // 发送邮件服务器 port，一般为465或587
        "verification": {
            "body_path": "./static/verification.html", // 验证码静态 html 文件路径
            "length": 6,                               // 验证码长度
            "expire_time": 120                         // 验证码过期时间
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

- 有关邮箱发送的问题，可以查看 [这个链接](https://wx.mail.qq.com/list/readtemplate?name=app_intro.html#/agreement/authorizationCode)
- 有关七牛云的问题，可以查看 [这个链接](https://www.bilibili.com/video/BV1fw411t7eU)

## Benchmark

### 测试环境

- CPU：Intel(R) Core(TM) i7-7820HQ CPU @ 2.90GHz
- RAM：16 GB
- OS：MacOS 13.4
- Go Version: Go 1.21

Server 仅输出 Warn 及以上级别的日志

### 测试工具

我们使用 `wrk` 作为HTTP压力测试工具。

### APIs

对一些可能并发量比较高的 API 进行测试，测试结果如下

#### 获取帖子列表

用户进入首页，会获取首页帖子

这里模拟用户请求，获取首页前 20 条帖子

```bash
wrk -t10 -c100 -d30s -s benchmark_post_list.lua http://127.0.0.1:1145
```

**测试结果**

|     | 平均值   | 标准差   | 最大值   | +/- 标准差 |
|-----|---------|---------|---------|-----------|
| 延迟 | 29.24ms | 13.55ms | 222.91ms| 93.66%    |
| RPS  | 357.88  | 82.98   | 484.00  | 84.85%   |

- 总请求数：106714
- 测试时长：30.05s
- 请求速率：3551.43 次/秒（Requests/sec）
- 吞吐量：153.22MB/sec

#### 获取帖子详情

获取单个帖子的详情（出现于大量用户点击同一篇帖子）

```bash
wrk -t10 -c100 -d30s -s benchmark_files/benchmark_post_detail.lua http://127.0.0.1:1145
```

**测试结果**

|     | 平均值   | 标准差   | 最大值   | +/- 标准差 |
|-----|---------|---------|---------|-----------|
| 延迟 | 5.37ms  | 7.06ms  | 131.53ms| 81.37%    |
| RPS | 5.15K   | 1.46K   | 8.57K   | 79.73%    |

- 总请求数：1537524
- 测试时长：30.02s
- 请求速率：51223.75 次/秒（Requests/sec）
- 吞吐量：53.69MB/sec

#### 搜索

**基于 bleve 的搜索**

```bash
wrk -t10 -c100 -d30s -s benchmark_files/benchmark_post_search_by_bleve.lua http://127.0.0.1:1145
```

**测试结果**

|     | 平均值   | 标准差   | 最大值   | +/- 标准差 |
|-----|---------|---------|---------|-----------|
| 延迟 | 22.40ms | 24.85ms | 301.43ms| 86.28%    |
| RPS | 611.74  | 118.72  | 1.02k   | 72.63%     |

- 总请求数：183106
- 测试时长：30.10s
- 请求速率：6083.96 次/秒（Requests/sec）
- 吞吐量：0.95GB/sec

**基于 elastic search 的搜索**

```bash
wrk -t10 -c100 -d30s -s benchmark_files/benchmark_post_search_by_es.lua http://127.0.0.1:1145
```

**测试结果**

|     | 平均值   | 标准差   | 最大值   | +/- 标准差 |
|-----|---------|---------|---------|-----------|
| 延迟 | 31.69ms | 20.01ms | 201.53ms| 74.50%    |
| RPS  | 332.40  | 82.59   | 585.00  | 72.00%     |

- 总请求数：99399
- 测试时长：30.05s
- 请求速率：3308.13 次/秒（Requests/sec）
- 吞吐量：286.68MB/sec

#### 创建评论

由于创建评论 API 1s 内可以创建 1k+ 的评论，故压测持续时间设置为 5s，线程、连接数均为 1

```bash
wrk -t1 -c1 -d5s -s benchmark_files/benchmark_comment_create.lua http://127.0.0.1:1145
```

**测试结果**

|     | 平均值    | 标准差   | 最大值  | +/- 标准差 |
|-----|---------|---------|--------|-----------|
| 延迟 | 821.80us| 376.76us| 6.99ms | 92.47%    |
| RPS |  1.24k  | 179.24  | 1.64k  | 72.55%    |

- 总请求数：6316
- 测试时长：5.10s
- 请求速率：1238.25 次/秒（Requests/sec）
- 吞吐量：796.88KB/sec

**需要注意的是**，即使这里的 rps 达到了 1.2k，但是消费者消费完这一批数据的耗时约为 30s，真正的 *消费速率* 应该在 200 左右

#### 获取评论列表

在根评论总数为 6k+ 的情况下，获取前 50 条评论：

```bash
wrk -t10 -c100 -d30s -s benchmark_files/benchmark_comment_list.lua http://127.0.0.1:1145
```

**测试结果**

|       | 平均值    | 标准差   | 最大值  | +/- 标准差 |
|-------|---------|---------|---------|------------|
| 延迟  | 35.04ms | 6.29ms | 81.18ms | 85.61%     |
| RPS   |  286.62  | 41.83  |  383.00  | 72.47%     |

- 总请求数：85711
- 测试时长：30.05s
- 请求速率：2852.52 次/秒（Requests/sec）
- 吞吐量：62.54MB/sec

#### 评论点赞

**单个用户给同一条评论点赞**

这种情况一般不会出现（除非是恶意用户，我们可以限流），这里只是为了测试：

```bash
wrk -t10 -c100 -d30s -s benchmark_files/benchmark_comment_like_single_user.lua http://127.0.0.1:1145
```

**测试结果**

|       | 平均值    | 标准差   | 最大值  | +/- 标准差 |
|-------|---------|---------|---------|------------|
| 延迟  | 57.13ms  | 10.24ms | 183.72ms | 90.97%     |
| RPS   | 175.92    | 25.98   | 232.00    | 78.30%     |

- 总请求数：52672
- 测试时长：30.06s
- 请求速率：1752.29 次/秒（Requests/sec）
- 吞吐量：17.68MB/sec

**多个用户给同一条评论点赞**

每次请求，随机选择 1000 个用户中的一个来发起点赞请求，这种情况更加符合正常情景，详情可以参阅 `benchmark_files/benchmark_comment_like_mult_users.lua`

```bash
wrk -t10 -c100 -d30s -s benchmark_files/benchmark_comment_like_mult_user.lua http://127.0.0.1:1145
```

**测试结果**

|       | 平均值    | 标准差   | 最大值  | +/- 标准差 |
|-------|---------|---------|---------|------------|
| 延迟  | 37.45ms  | 8.76ms  | 122.45ms | 81.02%     |
| RPS   | 268.04    | 45.74    | 1620     | 79.17%     |

- 总请求数：80184
- 测试时长：30.10s
- 请求速率：2663.57 次/秒（Requests/sec）
- 吞吐量：26.92MB/sec

这里再给出 **优化前** 的测试结果：

|       | 平均值    | 标准差   | 最大值  | +/- 标准差 |
|-------|---------|---------|---------|------------|
| 延迟  | 157.67ms  | 23.09ms | 318.21ms | 86.12%     |
| RPS   | 63.37     | 18.27   | 121.00   | 73.80%     |

- 总请求数：19010
- 测试时长：30.06s
- 请求速率：632.42 次/秒（Requests/sec）
- 吞吐量：6.38MB/sec

RPS 提升还是很明显的，延迟较于优化前也降低了近 5 倍

### 结论

基于以上的压力测试，可以得到以下几点结论：

1. 获取首页帖子的压力测试结果显示在高压力下，延迟平均 54.44ms，请求速率为 1840.18 次/秒。

2. 对于获取单个帖子的详情，压力测试结果表明，即使在高并发场景下，由于 **自适应热点检测** 的存在，延迟仍能保持在较低水平（平均延迟 5.37ms），并且具有高请求速度（51223.75 次/秒）。

3. 搜索的压力测试表明，无论是基于 bleve 的搜索还是基于 elastic search 的搜索，性能均相对稳健，延迟分别为 36.84ms 和 42.22ms，请求速率分别为 2719.17 次/秒和 2373.55 次/秒。

4. 创建评论的压力测试显示，服务器在 **高写入的环境下仍能保持可用性**，平均延迟为 821.80us，请求速率为 1238.25 次/秒，消费速率约为 200 条/秒

5. 获取评论列表的压力测试表明，尽管根评论总数为 6k+，但获取前 50 条评论的延迟处于较低范围（38.37ms），请求速率也可以达到 2852.52 次/秒

6. 点赞评论的压力测试表明，点赞 API 具有 2.6k+ 的 QPS，满足基本业务需求

----

该项目目前还有一些功能没实现，如用户模块的关注、私信，帖子的更新等

后续~~也许~~会补充