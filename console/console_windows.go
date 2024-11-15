// Package console sets console's behavior on init
package console

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/sirupsen/logrus"

	"github.com/FloatTech/ZeroBot-Plugin/kanban/banner"
)

var (
	//go:linkname modkernel32 golang.org/x/sys/windows.modkernel32
	modkernel32         *windows.LazyDLL
	procSetConsoleTitle = modkernel32.NewProc("SetConsoleTitleW")
)

//go:linkname errnoErr golang.org/x/sys/windows.errnoErr
func errnoErr(e syscall.Errno) error

func setConsoleTitle(title string) (err error) {
	var p0 *uint16
	p0, err = syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	r1, _, e1 := syscall.Syscall(procSetConsoleTitle.Addr(), 1, uintptr(unsafe.Pointer(p0)), 0, 0)
	if r1 == 0 {
		err = errnoErr(e1)
	}
	return
}

func init() {
	stdin := windows.Handle(os.Stdin.Fd())

	var mode uint32
	err := windows.GetConsoleMode(stdin, &mode)
	if err != nil {
		panic(err)
	}

	//mode &^= windows.ENABLE_QUICK_EDIT_MODE // 禁用快速编辑模式
	mode |= windows.ENABLE_EXTENDED_FLAGS // 启用扩展标志

	//mode &^= windows.ENABLE_MOUSE_INPUT    // 禁用鼠标输入
	mode |= windows.ENABLE_PROCESSED_INPUT // 启用控制输入

	//mode &^= windows.ENABLE_INSERT_MODE                           // 禁用插入模式
	//mode |= windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT // 启用输入回显&逐行输入

	//mode &^= windows.ENABLE_WINDOW_INPUT           // 禁用窗口输入
	//mode &^= windows.ENABLE_VIRTUAL_TERMINAL_INPUT // 禁用虚拟终端输入

	err = windows.SetConsoleMode(stdin, mode)
	if err != nil {
		panic(err)
	}

	stdout := windows.Handle(os.Stdout.Fd())
	err = windows.GetConsoleMode(stdout, &mode)
	if err != nil {
		panic(err)
	}

	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING // 启用虚拟终端处理
	mode |= windows.ENABLE_PROCESSED_OUTPUT            // 启用处理后的输出

	err = windows.SetConsoleMode(stdout, mode)
	// windows 带颜色 log 自定义格式
	logrus.SetFormatter(&logFormat{hasColor: err == nil})
	if err != nil {
		logrus.Warnln("VT100设置失败, 将以无色模式输出")
	}

	err = setConsoleTitle("ZeroBot-Plugin " + banner.Version + " " + banner.Copyright)
	if err != nil {
		panic(err)
	}
}
