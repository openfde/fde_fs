package personal_fusing

import (
	"fde_fs/logger"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

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
	root string
}

func (self *Ptfs) Init() {
	defer trace()()
	e := syscall.Chdir(self.root)
	if nil == e {
		self.root = "./"
	}
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

func (self *Ptfs) Mkdir(path string, mode uint32) (errc int) {
	defer trace(path, mode)(&errc)
	defer setuidgid()()
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
	defer setuidgid()()
	return self.open(path, flags, mode)
}

func (self *Ptfs) Open(path string, flags int) (errc int, fh uint64) {
	defer trace(path, flags)(&errc, &fh)
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
	path = filepath.Join(self.root, path)
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

const PathPrefix = ".local/share/openfde/media/0/"

func UmountPtfs() error {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("mount_query_home_failed", os.Getuid(), err)
		return err
	}
	androidDir := filepath.Join(home, PathPrefix)
	syscall.Setreuid(-1, 0)
	for _, dir := range androidDirList {
		logger.Info("umount_volumes", filepath.Join(androidDir, dir))
		err = syscall.Unmount(filepath.Join(androidDir, dir), 0)
		if err != nil {
			logger.Error("umount_volumes", filepath.Join(androidDir, dir), err)
			return err
		}
	}
	return nil
}

var homeDirNameMap map[string]string
var androidDirList, linuxDirList []string

func init() {
	homeDirNameMap = make(map[string]string)
	// Initialize the map with key-value pairs
	homeDirNameMap["Documents"] = "文档"
	homeDirNameMap["Downloads"] = "下载"
	homeDirNameMap["Music"] = "音乐"
	homeDirNameMap["Videos"] = "视频"
	homeDirNameMap["Pictures"] = "图片"
	linuxDirList = append(linuxDirList, "Documents")
	linuxDirList = append(linuxDirList, "Downloads")
	linuxDirList = append(linuxDirList, "Music")
	linuxDirList = append(linuxDirList, "Videos")
	linuxDirList = append(linuxDirList, "Pictures")

	androidDirList = append(androidDirList, "Documents")
	androidDirList = append(androidDirList, "Download")
	androidDirList = append(androidDirList, "Music")
	androidDirList = append(androidDirList, "Movies")
	androidDirList = append(androidDirList, "Pictures")
}

var fslock sync.Mutex

func get() bool {
	fslock.Lock()
	// Check if /proc/self/mounts contains "fde_ptfs" keyword
	mounts, err := ioutil.ReadFile("/proc/self/mounts")
	defer fslock.Unlock()
	if err != nil {
		logger.Error("read_mounts_file", nil, err)
		return false
	}
	if strings.Contains(string(mounts), "fde_ptfs") {
		logger.Info("fde_ptfs_found", nil)
		return true
	} else {
		logger.Info("fde_ptfs_not_found", nil)
		return false
	}

}

func getUserFolders() ([]string, []string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}
	var realLinuxDirList = make([]string, len(linuxDirList))
	var realAndroidList = make([]string, len(androidDirList))
	_, err = os.Stat(filepath.Join(homeDir, linuxDirList[0]))
	if err == nil { //en
		for i, v := range linuxDirList {
			realLinuxDirList[i] = filepath.Join(homeDir, v)
			realAndroidList[i] = filepath.Join(homeDir, ".local/share/openfde/media/0", androidDirList[i])
		}
	} else { //zh
		for i, v := range linuxDirList {
			realLinuxDirList[i] = filepath.Join(homeDir, homeDirNameMap[v])
			realAndroidList[i] = filepath.Join(homeDir, ".local/share/openfde/media/0", androidDirList[i])
		}
	}
	return realLinuxDirList, realAndroidList, nil

}

type MountArgs struct {
	Args   []string
	PassFS Ptfs
}

func MountPtfs() error {
	syscall.Umask(0)
	args := []string{"-o", "allow_other", "-o", "nonempty"}
	var argsList [][]string
	rlinuxList, randroidList, err := getUserFolders()
	if err != nil {
		logger.Error("mount_dir_fusing", nil, err)
		return err
	}

	for _, v := range randroidList {
		tmpList := make([]string, len(args))
		copy(tmpList, args)
		tmpList = append(tmpList, v)
		argsList = append(argsList, tmpList)
	}
	var mountArgs []MountArgs
	for i, v := range argsList {
		mountArgs = append(mountArgs, MountArgs{
			Args: v,
			PassFS: Ptfs{
				root: rlinuxList[i],
			},
		})
	}

	var wg sync.WaitGroup
	wg.Add(len(mountArgs))
	ch := make(chan struct{})
	hosts := make([]*fuse.FileSystemHost, len(mountArgs))
	for index, value := range mountArgs {
		go func(args []string, fs Ptfs, c chan struct{}) {
			defer wg.Done()
			hosts[index] = fuse.NewFileSystemHost(&fs)
			tr := hosts[index].Mount("", args)
			if !tr {
				logger.Error("mount_ptfsfuse_error", tr, nil)
				c <- struct{}{}
			}
		}(value.Args, value.PassFS, ch)
		time.Sleep(time.Second)
	}
	go func() {
		wg.Wait()        //waitting for all goroutine
		ch <- struct{}{} //unlock the main goroutine
	}()
	<-ch //block here
	logger.Info("mount_ptfs_exit", "exit")
	return nil
}
