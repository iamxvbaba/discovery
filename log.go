//go:build !windows

package discovery

import (
	lg "log"
	"os"
	"runtime"
	"syscall"
)

var Log *lg.Logger
var stdErrFile *os.File

func init() {
	f, err := os.OpenFile("discovery.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if nil != err {
		panic(err)
	}
	Log = lg.New(f, "[discovery]", lg.Ldate|lg.Ltime|lg.Lshortfile)

	// 把文件句柄保存到全局变量，避免被GC回收
	stdErrFile = f

	if err = syscall.Dup2(int(f.Fd()), int(os.Stderr.Fd())); err != nil {
		Log.Printf("syscall.Dup2:%v\n", err)
		return
	}
	// 内存回收前关闭文件描述符
	runtime.SetFinalizer(stdErrFile, func(fd *os.File) {
		_ = fd.Close()
	})
}
