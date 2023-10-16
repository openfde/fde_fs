package main


import (
	"fde_fs/logger"
	"flag"
	"sync"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/winfsp/cgofuse/fuse"
)

func validPermR(uid, duid, gid, dgid uint32, perm uint32) bool {
	var own uint32
	if uid == duid {
		own = (perm & uint32(0b111000000)) >> 6
		if own >= 4 {
			return true
		}
	} else if gid == dgid {
		own = (perm & uint32(0b000111000)) >> 3
	} else {
		own = perm & uint32(0b000000111)
	}

	if own >= 4 {
		return true
	}
	return false
}

func validPermW(uid, duid, gid, dgid int32, perm uint32) bool {
	var own uint32
	if uid == duid {
		own = (perm & uint32(0b111000000)) >> 6
		if own >= 4 {
			return true
		}
	} else if gid == dgid {
		own = (perm & uint32(0b000111000)) >> 3
	} else {
		own = perm & uint32(0b000000111)
	}

	if (own & 1 << 1) == 2 {
		return true
	}
	return false
}

const FSPrefix = "volumes"
const PathPrefix = "/volumes/"

func readProcess(pid uint32) {
	ioutil.ReadFile("/proc/" + fmt.Sprint(pid) + "/environ")
}

func Mount() (mArgs []MountArgs,err error) {
	syscall.Umask(0)
	mounts, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		logger.Error("mount_read_mountinfo", mounts, err)
		return
	}
	mountInfoByDevice := readDevicesAndMountPoint(mounts)
	files, err := ioutil.ReadDir("/dev/disk/by-uuid")
	if err != nil {
		logger.Error("mount_read_disk", mounts, err)
		return
	}
	logger.Info("mount_info_by_device", mountInfoByDevice)
	volumes, err := supplementVolume(files, mountInfoByDevice)
	if err != nil {
		logger.Error("mount_supplement_volume", mounts, err)
		return
	}

	_, err = os.Stat(PathPrefix)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(PathPrefix, os.ModeDir+0777)
			if err != nil {
				logger.Error("mount_mkdir_for_volumes", PathPrefix, err)
				return 
			}
		}
	}
	logger.Info("in_mount", volumes)
	for _, mountInfo := range volumes {
		path := PathPrefix + mountInfo.Volume
		_, err = os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				err = os.Mkdir(path, os.ModeDir+0750)
				if err != nil {
					logger.Error("mount_mkdir_for_volumes", mountInfo, err)
					return 
				}
			} else {
				logger.Error("mount_stat_volume", path, err)
				err = syscall.Unmount(path, 0)
				if err != nil {
					logger.Error("umount_volumes", path, err)
					return 
				}
			}
		}

		mArgs = append(mArgs,MountArgs{
			Args : []string{"-o", "allow_other", PathPrefix + mountInfo.Volume},
			PassFS: Ptfs{
				root: mountInfo.MountPoint,
			},
		})
	}
	return 
}
type MountArgs struct {
	Args []string
	PassFS Ptfs
}

type volumeAndMountPoint struct {
	Volume     string
	MountPoint string
	MountID    string
	// MountType  string
}

const LenFieldOfSelfMountInfo = 9

func readDevicesAndMountPoint(mounts []byte) map[string]volumeAndMountPoint {
	var mountInfoByDevice map[string]volumeAndMountPoint
	mountInfoByDevice = make(map[string]volumeAndMountPoint)
	lines := strings.Split(string(mounts), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		//below is a line example of the mountinfo
		//35 29 8:5 / /data rw,relatime shared:7 - ext4 /dev/sda5 rw
		//807 790 7:1 / /var/lib/waydroid/rootfs/vendor ro,relatime shared:446 - ext4 /dev/loop1 ro
		if len(fields) < LenFieldOfSelfMountInfo {
			continue
		}
		//只有第3个元素，仅仅包含/才是原始挂载
		if len(fields[3]) > 1 {
			continue
		}
		//文件系统不是ext4的过滤掉
		if fields[8] != "ext4" {
			continue
		}
		//不需要loop设备
		if strings.Contains(fields[9], "loop") {
			continue
		}
		mountPoint := fields[4]
		if mountPoint == "/boot" {
			continue
		}
		mountID := fields[0]
		if value, exist := mountInfoByDevice[fields[9]]; exist {
			srcMountID, err := strconv.Atoi(value.MountID)
			if err != nil {
				continue
			}
			currentMountID, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}
			if currentMountID > srcMountID {
				mountPoint = value.MountPoint
				mountID = value.MountID
			}
		}
		mountInfoByDevice[fields[9]] = volumeAndMountPoint{
			MountPoint: mountPoint,
			MountID:    mountID,
		}
	}
	return mountInfoByDevice

}

func supplementVolume(files []fs.FileInfo, mountInfoByDevice map[string]volumeAndMountPoint) (map[string]volumeAndMountPoint, error) {
	var volumesByDevice map[string]volumeAndMountPoint
	volumesByDevice = make(map[string]volumeAndMountPoint)
	for _, v := range files {
		name, err := os.Readlink("/dev/disk/by-uuid/" + v.Name())
		if err != nil {
			logger.Error("read_volumes", name, err)
			return nil, err
		}
		name = strings.Replace(name, "../..", "/dev", 1)
		if value, exist := mountInfoByDevice[name]; exist {
			volumesByDevice[name] = volumeAndMountPoint{
				Volume:     v.Name(),
				MountPoint: value.MountPoint,
				MountID:    value.MountID,
			}
		}
	}
	return volumesByDevice, nil
}

func UmountAllVolumes() error {
	entries, err := os.ReadDir(PathPrefix)
	if err != nil {
		return err
	}
	syscall.Setreuid(-1, 0)
	for _, volume := range entries {
		path := PathPrefix + volume.Name()
		err = syscall.Unmount(path, 0)
		if err != nil {
			logger.Error("umount_volumes", path, err)
		}
	}
	return nil
}

func main() {
	var umount, mount, help bool
	flag.BoolVar(&mount, "m", false, "-m")
	flag.BoolVar(&umount, "u", false, "-u")
	flag.BoolVar(&help, "h", false, "-h")
	flag.Parse()
	if help {
		fmt.Println("fde_fs:")
		fmt.Println("\t-h: help")
		fmt.Println("\t-u: umount all volumes")
		fmt.Println("\t-m: mount all volumes")
		return
	}

	if umount {
		logger.Info("umount_all_volumes","umount")
		err := UmountAllVolumes()
		if err != nil {
			logger.Error("umount_failed",nil,err)
		}
		return
	}
	mountArgs,err := Mount()
	if err != nil {
		os.Exit(1)
	}
	var wg sync.WaitGroup
	wg.Add(len(mountArgs))
	ch := make(chan struct{})
	for _,value := range mountArgs{
		go func(args []string,  fs Ptfs, c chan struct{}) {
			defer wg.Done()
			var host *fuse.FileSystemHost
			host = fuse.NewFileSystemHost(&fs)
			tr := host.Mount("", args)
			if !tr {
				logger.Error("mount_fuse_error", tr, nil)
				c <- struct{}{}
			}
		}(value.Args, value.PassFS,ch)
	}
	go func() {
		wg.Wait()
		ch <- struct{}{}
	}()
	<-ch
	logger.Info("mount_exit","exit")
}

