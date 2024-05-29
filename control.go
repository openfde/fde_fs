package main

import (
	"fde_fs/logger"
	"os"
	"strconv"
	"syscall"

	"github.com/winfsp/cgofuse/fuse"
)

func (self *Ptfs) isHostNS() bool {
	_, _, pid := fuse.Getcontext()
	ns, err := self.readNS(strconv.Itoa(pid))
	if err != nil {
		return false
	}
	if self.ns == 0 {
		self.recordNameSpace()
	}
	return ns == self.ns
}

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

func (self *Ptfs) recordNameSpace() {
	pid := os.Getpid()
	var err error
	self.ns, err = self.readNS(strconv.Itoa(pid))
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
