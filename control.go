package main

import (
	"bytes"
	"fde_fs/logger"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func (self *Ptfs) readNS(pid string) (nsid uint64, err error) {
	file := "/proc/" + pid + "/ns/pid"
	fd, err := os.Open(file)
	if err != nil {
		logger.Error("read_name_space_fs", nil, err)
		return
	}
	defer fd.Close()
	var stat syscall.Stat_t
	syscall.Fstat(int(fd.Fd()), &stat)
	nsid = stat.Ino
	return
}

func (self *Ptfs) isHomeFDE() bool {
	fmt.Println("in is home fde")
	list := strings.Split(self.original, "/")
	if len(list) < 4 {
		return false
	}
	fmt.Println("in is home fde args", list[1], list[3])
	if list[1] == "home" && list[3] == "fde" {
		return true
	}
	return false
}

func (self *Ptfs) recordNameSpace() {
	psCmd := exec.Command("ps", "-ef")
	grepCmd := exec.Command("grep", "fde_fs")
	xgrepCmd := exec.Command("grep", "-v", "grep")
	// 将 ps 命令的输出传递给 grep 命令进行过滤
	var output bytes.Buffer
	grepCmd.Stdin, _ = psCmd.StdoutPipe()
	xgrepCmd.Stdin, _ = grepCmd.StdoutPipe()
	xgrepCmd.Stdout = &output
	err := psCmd.Start()
	if err != nil {
		return
	}
	err = grepCmd.Start()
	if err != nil {
		return
	}
	err = xgrepCmd.Start()
	if err != nil {
		return
	}
	err = psCmd.Wait()
	if err != nil {
		return
	}
	grepCmd.Wait()
	xgrepCmd.Wait()
	// 解析 grep 命令的输出

	fields := strings.Fields(output.String())
	if len(fields) < 2 {
		return
	}
	self.ns, err = self.readNS(fields[1])
	if err != nil {
		logger.Error("record_ns", nil, err)
	}
	return

}

/*
// Flush flushes cached file data.
// The FileSystemBase implementation returns -ENOSYS.

	func (self *Ptfs) Flush(path string, fh uint64) int {
		syscall.Flush
		return -ENOSYS
	}

// Lock performs a file locking operation.
// The FileSystemBase implementation returns -ENOSYS.

	func (self *Ptfs) Lock(path string, cmd int, lock *Lock_t, fh uint64) int {
		return -ENOSYS
	}

// Fsyncdir synchronizes directory contents.
// The FileSystemBase implementation returns -ENOSYS.

	func (self *Ptfs) Fsyncdir(path string, datasync bool, fh uint64) int {
		syscall.Fsyncd
		return -ENOSYS
	}

// Setxattr sets extended attributes.
// The FileSystemBase implementation returns -ENOSYS.

	func (self *Ptfs) Setxattr(path string, name string, value []byte, flags int) int {
		return -ENOSYS
	}

// Getxattr gets extended attributes.
// The FileSystemBase implementation returns -ENOSYS.

	func (self *Ptfs) Getxattr(path string, name string) (int, []byte) {
		path = filepath.Join(self.root, path)
		var buffer []byte
		sz, err := syscall.Getxattr(path, name, buffer)
		return errno(err), buffer
	}

// Removexattr removes extended attributes.
// The FileSystemBase implementation returns -ENOSYS.

	func (self *Ptfs) Removexattr(path string, attr string) int {
		path = filepath.Join(self.root, path)
		return errno(syscall.Removexattr(path, attr))
	}

// Listxattr lists extended attributes.
// The FileSystemBase implementation returns -ENOSYS.

	func (self *Ptfs) Listxattr(path string, fill func(name string) bool) int {
		path = filepath.Join(self.root, path)
		var buffer []byte
		syscall.Listxattr(path,)
		return errno(syscall.Listxattr(path, attr))
	}
*/
