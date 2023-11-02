package logger

import (
	"os"

	"github.com/natefinch/lumberjack"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.SugaredLogger

func InitLogger() {
	var core zapcore.Core
	var (
		path       = viper.GetString("logger.path")
		maxSize    = viper.GetInt("logger.max_size")
		maxBackups = viper.GetInt("logger.max_backups")
		compress   = viper.GetBool("logger.compress")
		console    = viper.GetBool("logger.console")
	)
	opinions := make([]zap.Option, 0, 1)

	if viper.GetBool("server.develop_mode") {
		core = zapcore.NewCore(getEncoder(zap.NewDevelopmentEncoderConfig()),
			getWriter(path, maxSize, maxBackups, compress, console),
			zap.DebugLevel)
		opinions = append(opinions, zap.AddCaller(), zap.AddCallerSkip(1))
	} else {
		var level zapcore.Level = zapcore.Level(viper.GetInt("logger.level"))
		core = zapcore.NewCore(getEncoder(zap.NewProductionEncoderConfig()),
			getWriter(path, maxSize, maxBackups, compress, console),
			level)
	}

	native_logger := zap.New(core, opinions...)
	logger = native_logger.Sugar()

	Infof("Initializing logger successfully")
}

// func GetLogger() *zap.SugaredLogger {
// 	return logger
// }

func Debugf(template string, args ...any) {
	logger.Debugf(template, args...)
}

func Infof(template string, args ...any) {
	logger.Infof(template, args...)
}

func Warnf(template string, args ...any) {
	logger.Warnf(template, args...)
}

func Errorf(template string, args ...any) {
	logger.Errorf(template, args...)
}

func ErrorWithStack(err error) {
	logger.Errorf("%T:\nstack trace:\n%+v", errors.Cause(err), err)
}

func getEncoder(config zapcore.EncoderConfig) zapcore.Encoder {
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncodeLevel = zapcore.CapitalLevelEncoder // 日志级别大写，如 DEBUG

	return zapcore.NewConsoleEncoder(config)
}

func getWriter(path string, maxSize, maxBackups int, compress, console bool) zapcore.WriteSyncer {
	lumberjackLogger := &lumberjack.Logger{ // 分片机制
		Filename:   path,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		Compress:   compress,
	}
	out := make([]zapcore.WriteSyncer, 0, 2)
	out = append(out, zapcore.AddSync(lumberjackLogger))
	if console {
		out = append(out, os.Stdout)
	}
	return zapcore.NewMultiWriteSyncer(out...)
}
