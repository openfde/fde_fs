package main

import (
	"bytes"
	"fde_fs/logger"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/winfsp/cgofuse/examples/shared"
	"github.com/winfsp/cgofuse/fuse"
)

func errno(err error) int {
	if nil != err {
		return -int(err.(syscall.Errno))
	} else {
		return 0
	}
}

func trace(vals ...interface{}) func(vals ...interface{}) {
	uid, gid, _ := fuse.Getcontext()
	return shared.Trace(1, fmt.Sprintf("[uid=%v,gid=%v]", uid, gid), vals...)
}

type Ptfs struct {
	fuse.FileSystemBase
	root     string
	original string
	ns       uint64
}

func (self *Ptfs) Init() {
	defer trace()()
	e := syscall.Chdir(self.root)
	self.original = self.root
	if nil == e {
		self.root = "./"
	}
}

// Destroy is called when the file system is destroyed.
// The FileSystemBase implementation does nothing.
func (self *Ptfs) Destroy() {
}

// Access checks file access permissions.
// The FileSystemBase implementation returns -ENOSYS.
func (self *Ptfs) Access(path string, mask uint32) int {
	path = filepath.Join(self.root, path)
	return errno(syscall.Access(path, mask))
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
func (self *Ptfs) Statfs(path string, stat *fuse.Statfs_t) (errc int) {
	defer trace(path)(&errc, stat)
	path = filepath.Join(self.root, path)
	stgo := syscall.Statfs_t{}
	errc = errno(syscall_Statfs(path, &stgo))
	copyFusestatfsFromGostatfs(stat, &stgo)
	return
}

func (self *Ptfs) Mknod(path string, mode uint32, dev uint64) (errc int) {
	defer trace(path, mode, dev)(&errc)
	defer setuidgid()()
	path = filepath.Join(self.root, path)
	return errno(syscall.Mknod(path, mode, int(dev)))
}

func (self *Ptfs) Mkdir(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	defer setuidgid()()
	path = filepath.Join(self.root, path)
	return errno(syscall.Mkdir(path, mode))
}

func (self *Ptfs) Unlink(path string) (errc int) {
	defer trace(path)(&errc)
	defer setuidgid()()
	path = filepath.Join(self.root, path)
	return errno(syscall.Unlink(path))
}

func (self *Ptfs) Rmdir(path string) (errc int) {
	defer trace(path)(&errc)
	defer setuidgid()()
	path = filepath.Join(self.root, path)
	return errno(syscall.Rmdir(path))
}

func (self *Ptfs) Link(oldpath string, newpath string) (errc int) {
	defer trace(oldpath, newpath)(&errc)
	defer setuidgid()()
	oldpath = filepath.Join(self.root, oldpath)
	newpath = filepath.Join(self.root, newpath)
	return errno(syscall.Link(oldpath, newpath))
}

func (self *Ptfs) Symlink(target string, newpath string) (errc int) {
	defer trace(target, newpath)(&errc)
	defer setuidgid()()
	newpath = filepath.Join(self.root, newpath)
	return errno(syscall.Symlink(target, newpath))
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

func (self *Ptfs) Readlink(path string) (errc int, target string) {
	defer trace(path)(&errc, &target)
	defer setuidgid()()
	path = filepath.Join(self.root, path)
	buff := [1024]byte{}
	n, e := syscall.Readlink(path, buff[:])
	if nil != e {
		return errno(e), ""
	}
	return 0, string(buff[:n])
}

func (self *Ptfs) Rename(oldpath string, newpath string) (errc int) {
	defer trace(oldpath, newpath)(&errc)
	defer setuidgid()()
	oldpath = filepath.Join(self.root, oldpath)
	newpath = filepath.Join(self.root, newpath)
	return errno(syscall.Rename(oldpath, newpath))
}

func (self *Ptfs) Chmod(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	defer setuidgid()()
	path = filepath.Join(self.root, path)
	return errno(syscall.Chmod(path, mode))
}

func (self *Ptfs) Chown(path string, uid uint32, gid uint32) (errc int) {
	defer trace(path, uid, gid)(&errc)
	defer setuidgid()()
	path = filepath.Join(self.root, path)
	return errno(syscall.Lchown(path, int(uid), int(gid)))
}

func (self *Ptfs) Utimens(path string, tmsp1 []fuse.Timespec) (errc int) {
	defer trace(path, tmsp1)(&errc)
	path = filepath.Join(self.root, path)
	tmsp := [2]syscall.Timespec{}
	tmsp[0].Sec, tmsp[0].Nsec = tmsp1[0].Sec, tmsp1[0].Nsec
	tmsp[1].Sec, tmsp[1].Nsec = tmsp1[1].Sec, tmsp1[1].Nsec
	return errno(syscall.UtimesNano(path, tmsp[:]))
}

func (self *Ptfs) isHomeFDE() bool {
	list := strings.Split(self.original, "/")
	if len(list) < 4 {
		return false
	}
	if list[1] == "home" && list[3] == "fde" {
		return true
	}
	return false
}

func (self *Ptfs) Create(path string, flags int, mode uint32) (errc int, fh uint64) {
	defer trace(path, flags, mode)(&errc, &fh)
	// defer setuidgid()()
	uid, _, _ := fuse.Getcontext()
	path = filepath.Join(self.root, path)
	if self.isHomeFDE() {
		syscall.Chown(path, int(uid), 10038)
	}
	return self.open(path, flags, mode)
}

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

func (self *Ptfs) Open(path string, flags int) (errc int, fh uint64) {
	defer trace(path, flags)(&errc, &fh)
	if self.isHostNS() {
		var st syscall.Stat_t
		syscall.Stat(path, &st)
		var dstSt fuse.Stat_t
		copyFusestatFromGostat(&dstSt, &st)
		uid, gid, _ := fuse.Getcontext()
		if !validPermR(uint32(uid), st.Uid, gid, st.Gid, dstSt.Mode) {
			//-1 means no permission
			info := fmt.Sprint(uid, "=uid, ", st.Uid, "=fileuid, ", gid, "=gid", st.Gid, "=filegid")
			logger.Info("open_dir", info)
			return -int(syscall.EACCES), 0
		}
	} else {
		//is fde
		//todo based as only one instance of fde, should consider multiple instances of fde
		//read the permission allowd list to decide whether the uid have permission to do

	}
	return self.open(path, flags, 0)
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

func (self *Ptfs) open(path string, flags int, mode uint32) (errc int, fh uint64) {
	path = filepath.Join(self.root, path)
	//todo controll the permission by ourself policy
	//identity where the request from , android or linux
	f, e := syscall.Open(path, flags, mode)
	if nil != e {
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (self *Ptfs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	defer trace(path, fh)(&errc, stat)
	defer setuidgid()()
	stgo := syscall.Stat_t{}
	if ^uint64(0) == fh {
		path = filepath.Join(self.root, path)
		errc = errno(syscall.Lstat(path, &stgo))
	} else {
		errc = errno(syscall.Fstat(int(fh), &stgo))
	}
	copyFusestatFromGostat(stat, &stgo)
	return
}

func (self *Ptfs) Truncate(path string, size int64, fh uint64) (errc int) {
	defer trace(path, size, fh)(&errc)
	defer setuidgid()()
	if ^uint64(0) == fh {
		path = filepath.Join(self.root, path)
		errc = errno(syscall.Truncate(path, size))
	} else {
		errc = errno(syscall.Ftruncate(int(fh), size))
	}
	return
}

func (self *Ptfs) Read(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer trace(path, buff, ofst, fh)(&n)
	defer setuidgid()()
	n, e := syscall.Pread(int(fh), buff, ofst)
	if nil != e {
		return errno(e)
	}
	return n
}

func (self *Ptfs) Write(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer trace(path, buff, ofst, fh)(&n)
	defer setuidgid()()
	n, e := syscall.Pwrite(int(fh), buff, ofst)
	if nil != e {
		return errno(e)
	}
	return n
}

func (self *Ptfs) Release(path string, fh uint64) (errc int) {
	defer trace(path, fh)(&errc)
	return errno(syscall.Close(int(fh)))
}

func (self *Ptfs) Fsync(path string, datasync bool, fh uint64) (errc int) {
	defer trace(path, datasync, fh)(&errc)
	return errno(syscall.Fsync(int(fh)))
}

func (self *Ptfs) Opendir(path string) (errc int, fh uint64) {
	defer trace(path)(&errc, &fh)
	if self.isHostNS() {
		var st syscall.Stat_t
		syscall.Stat(path, &st)
		var dstSt fuse.Stat_t
		copyFusestatFromGostat(&dstSt, &st)
		uid, gid, _ := fuse.Getcontext()
		if !validPermR(uint32(uid), st.Uid, gid, st.Gid, dstSt.Mode) {
			//-1 means no permission
			info := fmt.Sprint(uid, "=uid, ", st.Uid, "=fileuid, ", gid, "=gid", st.Gid, "=filegid")
			logger.Info("open_dir", info)
			return -int(syscall.EACCES), 0
		}
	} else {
		//is fde
		//todo based as only one instance of fde, should consider multiple instances of fde
		//read the permission allowd list to decide whether the uid have permission to do

	}

	if self.original == "/" {
		list := strings.Split(path, "/")
		if len(list) >= 2 {
			if list[1] == FSPrefix {
				return int(syscall.ENOENT), 1
			}
		}
	}
	f, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if nil != e {
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (self *Ptfs) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) (errc int) {
	defer trace(path, fill, ofst, fh)(&errc)
	path = filepath.Join(self.root, path)
	file, e := os.Open(path)
	if nil != e {
		return errno(e)
	}
	defer file.Close()
	nams, e := file.Readdirnames(0)
	if nil != e {
		return errno(e)
	}
	nams = append([]string{".", ".."}, nams...)
	for _, name := range nams {
		if self.original == "/" {
			if name == FSPrefix {
				continue
			}
		}
		if !fill(name, nil, 0) {
			break
		}
	}
	return 0
}

func (self *Ptfs) Releasedir(path string, fh uint64) (errc int) {
	defer trace(path, fh)(&errc)
	return errno(syscall.Close(int(fh)))
}
