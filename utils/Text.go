package utils

import "github.com/mattn/go-runewidth"

// Truncate 截断字符串(按照打印出来的长度计算，相对于ascii字符宽度)
func Truncate(title string, maxlen int) string {
	return runewidth.Truncate(title, maxlen, "...")
}
