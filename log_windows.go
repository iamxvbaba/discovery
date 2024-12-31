//go:build windows

package discovery

import (
	lg "log"
	"os"
)

var Log *lg.Logger

func init() {
	f, err := os.OpenFile("std_out.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if nil != err {
		panic(err)
	}
	Log = lg.New(f, "[discovery]", lg.Ldate|lg.Ltime|lg.Lshortfile)
}
