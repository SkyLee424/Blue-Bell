# Makefile

# 设置 Go 可执行文件名
EXECUTABLE := bluebell

SERVER_EXECUTABLE := bluebell-server

# 设置输出目录
BUILD_DIR := bin

# 设置 Go 编译器
GO := go

# 设置配置文件路径
CONFIG_PATH = ./config/config_test.json

.PHONY: all clean

all: clean fmt init build run

fmt:
	swag fmt

init:
	swag init

build:
	# 编译 Go 程序
	$(GO) build -o $(BUILD_DIR)/$(EXECUTABLE) main.go

run:
	./$(BUILD_DIR)/$(EXECUTABLE) -c $(CONFIG_PATH)

clean:
	rm -rf $(BUILD_DIR)

linux:
	GOOS=linux GOARCH=amd64 $(GO) build -o $(SERVER_EXECUTABLE)