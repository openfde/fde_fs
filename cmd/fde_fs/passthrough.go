//go:build darwin || freebsd || netbsd || openbsd || linux
// +build darwin freebsd netbsd openbsd linux

/*
 * passthrough.go
 *
 * Copyright 2017-2022 Bill Zissimopoulos
 */
/*
 * This file is part of Cgofuse.
 *
 * It is licensed under the MIT license. The full license text can be found
 * in the License.txt file at the root of this project.
 */

package main

import (
	"fde_fs/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/winfsp/cgofuse/examples/shared"
	"github.com/winfsp/cgofuse/fuse"
)

func trace(vals ...interface{}) func(vals ...interface{}) {
	uid, gid, _ := fuse.Getcontext()
	return shared.Trace(1, fmt.Sprintf("[uid=%v,gid=%v]", uid, gid), vals...)
}

func errno(err error) int {
	if nil != err {
		return -int(err.(syscall.Errno))
	} else {
		return 0
	}
}

var (
	_host *fuse.FileSystemHost
)

type Ptfs struct {
	fuse.FileSystemBase
	original string
	ns       uint64
	root     string
}

func (self *Ptfs) Init() {
	defer trace()()
	self.original = self.root
	// e := syscall.Chdir(self.root)
	//	if nil == e {
	//		self.root = "./"
	//	}
}

// Destroy is called when the file system is destroyed.
// The FileSystemBase implementation does nothing.
func (self *Ptfs) Destroy() {
}

// Access checks file access permissions.
// The FileSystemBase implementation returns -ENOSYS.
func (self *Ptfs) Access(path string, mask uint32) int {
	path = filepath.Join(self.root, path)
	uid, gid, _ := fuse.Getcontext()
	rpath := path
	if self.isHostNS() {
		//accessing openfde
		if strings.Contains(self.original, LocalOpenfde) {
			dirList := strings.Split(self.original, LocalOpenfde)
			if len(dirList) >= 2 {
				rpath = dirList[0]
			}
			var st syscall.Stat_t
			syscall.Stat(rpath, &st)
			var dstSt fuse.Stat_t
			copyFusestatFromGostat(&dstSt, &st)
			if !validPermR(uint32(uid), st.Uid, gid, st.Gid, dstSt.Mode) {
				//-1 means no permission
				info := fmt.Sprint(uid, "=uid, ", st.Uid, "=fileuid, ", gid, "=gid", st.Gid, "=filegid")
				logger.Info("open_dir", info)
				return -int(syscall.EACCES)
			} else {
				//more mask need to checking , not just reading
				return 0
			}
		}
	} else {
		//from android
		//todo based as only one instance of fde, should consider multiple instances of fde
		//read the permission allowd list to decide whether the uid have permission to do

	}
	return errno(syscall.Access(path, mask))
}

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

func (self *Ptfs) isOpenfdeFileSystem() bool {
	if strings.Contains(self.original, LocalOpenfde) {
		dirList := strings.Split(self.original, LocalOpenfde)
		return len(dirList) >= 2
	}
	return false
}

func (self *Ptfs) haveWPerm() bool {
	dirList := strings.Split(self.original, LocalOpenfde)
	home := dirList[0]
	var st syscall.Stat_t
	syscall.Stat(home, &st)
	var dstSt fuse.Stat_t
	copyFusestatFromGostat(&dstSt, &st)
	uid, gid, _ := fuse.Getcontext()
	if !validPermW(uint32(uid), st.Uid, gid, st.Gid, dstSt.Mode) {
		//-1 means no permission
		info := fmt.Sprint(uid, "=uid, ", st.Uid, "=fileuid, ", gid, "=gid", st.Gid, "=filegid", "for_path", home)
		logger.Warn("judge_w_permission", info)
		return false
	}
	return true
}

func (self *Ptfs) Mkdir(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	if self.isHostNS() && self.isOpenfdeFileSystem() {
		if !self.haveWPerm() {
			return -int(syscall.EACCES)
		}
		var st syscall.Stat_t
		var dstSt fuse.Stat_t
		//get the uid of the parent dir of the target
		if filepath.Dir(filepath.Join(self.root, path)) == self.root {
			//stat the Ringtones to get the uid and gid as the
			syscall.Stat(filepath.Join(self.root, "Ringtones"), &st)
		} else {
			syscall.Stat(filepath.Dir(filepath.Join(self.root, path)), &st)
		}
		copyFusestatFromGostat(&dstSt, &st)
		defer syscall.Chown(filepath.Join(self.root, path), int(dstSt.Uid), int(dstSt.Gid))
	} else {
		defer setuidgid()()
	}
	path = filepath.Join(self.root, path)
	return errno(syscall.Mkdir(path, mode))
}

func (self *Ptfs) Unlink(path string) (errc int) {
	defer trace(path)(&errc)
	path = filepath.Join(self.root, path)
	return errno(syscall.Unlink(path))
}

func (self *Ptfs) Rmdir(path string) (errc int) {
	defer trace(path)(&errc)
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

func (self *Ptfs) Readlink(path string) (errc int, target string) {
	defer trace(path)(&errc, &target)
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
	path = filepath.Join(self.root, path)
	return errno(syscall.Chmod(path, mode))
}

func (self *Ptfs) Chown(path string, uid uint32, gid uint32) (errc int) {
	defer trace(path, uid, gid)(&errc)
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

func (self *Ptfs) Create(path string, flags int, mode uint32) (errc int, fh uint64) {
	defer trace(path, flags, mode)(&errc, &fh)
	if self.isHostNS() && self.isOpenfdeFileSystem() {
		if !self.haveWPerm() {
			return -int(syscall.EACCES), 0
		}
		var st syscall.Stat_t
		var dstSt fuse.Stat_t
		//get the uid of the parent dir of the target
		if filepath.Dir(filepath.Join(self.root, path)) == self.root {
			//stat the Ringtones to get the uid and gid as the
			syscall.Stat(filepath.Join(self.root, "Ringtones"), &st)
		} else {
			syscall.Stat(filepath.Dir(filepath.Join(self.root, path)), &st)
		}
		copyFusestatFromGostat(&dstSt, &st)
		defer syscall.Chown(filepath.Join(self.root, path), int(dstSt.Uid), int(dstSt.Gid))
	} else {
		defer setuidgid()()
	}
	return self.open(path, flags, mode)
}

func (self *Ptfs) Open(path string, flags int) (errc int, fh uint64) {
	defer trace(path, flags)(&errc, &fh)
	var rpath string
	if self.isHostNS() {
		//accessing openfde
		if strings.Contains(self.original, LocalOpenfde) {
			dirList := strings.Split(self.original, LocalOpenfde)
			if len(dirList) >= 2 {
				//the permission of the files, which included by openfde, uses the permission of home selfs.
				rpath = dirList[0]
			}
		} else {
			rpath = filepath.Join(self.root, path)
		}
		var st syscall.Stat_t
		syscall.Stat(rpath, &st)
		var dstSt fuse.Stat_t
		copyFusestatFromGostat(&dstSt, &st)
		uid, gid, _ := fuse.Getcontext()
		if !validPermR(uint32(uid), st.Uid, gid, st.Gid, dstSt.Mode) {
			//-1 means no permission
			info := fmt.Sprint(uid, "=uid, ", st.Uid, "=fileuid, ", gid, "=gid", st.Gid, "=filegid")
			logger.Info("open", info)
			return -int(syscall.EACCES), 0
		}

	} else {
		//read the uid of linux user from lxc namespace?
		//todo should consider multiple instances of openfde
		//read the permission allowd list to decide whether the uid have permission to do

	}

	return self.open(path, flags, 0)
}

func (self *Ptfs) open(path string, flags int, mode uint32) (errc int, fh uint64) {
	path = filepath.Join(self.root, path)
	f, e := syscall.Open(path, flags, mode)
	if nil != e {
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (self *Ptfs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	defer trace(path, fh)(&errc, stat)
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
	n, e := syscall.Pread(int(fh), buff, ofst)
	if nil != e {
		return errno(e)
	}
	return n
}

func (self *Ptfs) Write(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer trace(path, buff, ofst, fh)(&n)
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
	path = filepath.Join(self.original, path)
	uid, gid, _ := fuse.Getcontext()
	rpath := path
	if self.isHostNS() {
		//accessing openfde
		if strings.Contains(self.original, LocalOpenfde) {
			dirList := strings.Split(self.original, LocalOpenfde)
			if len(dirList) >= 2 {
				rpath = dirList[0]
			}
		}
		var st syscall.Stat_t
		syscall.Stat(rpath, &st)
		var dstSt fuse.Stat_t
		copyFusestatFromGostat(&dstSt, &st)
		if !validPermR(uint32(uid), st.Uid, gid, st.Gid, dstSt.Mode) {
			//-1 means no permission
			info := fmt.Sprint(uid, "=uid, ", st.Uid, "=fileuid, ", gid, "=gid", st.Gid, "=filegid")
			logger.Info("open_dir", info)
			return -int(syscall.EACCES), 0
		}
	} else {
		//from android
		//todo based as only one instance of fde, should consider multiple instances of fde
		//read the permission allowd list to decide whether the uid have permission to do

	}
	//avoid the recursive loop of the /volumes directory
	if self.original == "/" {
		list := strings.Split(path, "/")
		if len(list) >= 2 {
			if list[1] == FSPrefix {
				return -int(syscall.ENOENT), 1
			}
		}
	}
	f, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if nil != e {
		return errno(e), ^uint64(0)
	}
	return 0, uint64(f)
}

func (self *Ptfs) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) (errc int) {
	defer trace(path, fill, ofst, fh)(&errc)
	path = filepath.Join(self.original, path)
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
