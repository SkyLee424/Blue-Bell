package utils

import (
	"strconv"
	"unicode/utf8"
)

func Substr(s string, start int, length int) string {
	// 将字符串转换为rune切片以处理Unicode字符
	runes := []rune(s)

	// 获取字符串长度
	strLen := utf8.RuneCountInString(s)

	// 确保起始位置和子字符串长度在有效范围内
	if start < 0 || start >= strLen || length <= 0 {
		return ""
	}

	// 获取子字符串的起始和结束位置
	substrStart := start
	substrEnd := start + length

	if substrEnd > strLen {
		substrEnd = strLen // 限制最大长度
	}

	// 从rune切片中提取子字符串
	substr := runes[substrStart:substrEnd]

	// 将rune切片转换为字符串并返回
	return string(substr)
}

func ConvertInt64SliceToStringSlice(arr []int64) []string {
	res := make([]string, 0, len(arr))
	for _, commentID := range arr {
		arrtr := strconv.FormatInt(commentID, 10)
		res = append(res, arrtr)
	}

	return res
}
