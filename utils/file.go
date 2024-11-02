package utils

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// Exists 检查给定路径上的文件或目录是否存在。
// 参数:
//
//	path: 需要检查的文件或目录的路径。
//
// 返回值:
//
//	bool: 如果文件或目录存在，则返回true；否则返回false。
func Exists(path string) bool {
	// 尝试获取路径的状态信息。
	_, err := os.Stat(path)
	// 如果没有错误，说明路径存在，返回true。
	if err == nil {
		return true
	}
	// 如果错误类型为文件或目录不存在，则返回false。
	if os.IsNotExist(err) {
		return false
	}
	// 如果遇到其他错误，记录日志信息并返回false。
	log.Infof("unable to stat path %q; %v", path, err)
	return false
}

// RedBytes 读取指定文件的内容并以字节切片的形式返回。
// 参数 c 是文件路径。
// 返回值是文件内容的字节切片和一个错误（如果有的话）。
func RedBytes(c string) ([]byte, error) {
	return os.ReadFile(c)
}

// WriteBytes 将字节切片的数据写入指定文件。
// 参数 c 是文件路径，data 是要写入的数据。
// 返回值是一个错误（如果有的话）。
// 函数使用 0644 权限模式创建或覆盖文件，这意味着文件所有者可以读写该文件，而其他用户只能读取。
func WriteBytes(c string, data []byte) error {
	return os.WriteFile(c, data, 0644)
}
