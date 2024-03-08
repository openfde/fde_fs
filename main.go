package main

import (
	"fde_fs/logger"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

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

func validPermW(uid, duid, gid, dgid uint32, perm uint32) bool {
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

func ConstructMountArgs() (mArgs []MountArgs, err error) {
	syscall.Umask(0)
	mounts, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		logger.Error("mount_read_mountinfo", mounts, err)
		return
	}
	mountInfoByDevice := readDevicesAndMountPoint(mounts)
	files, err := ioutil.ReadDir("/dev/disk/by-partlabel")
	if err != nil {
		logger.Error("mount_read_disk", mounts, err)
		return
	}
	logger.Info("mount_info_by_device", mountInfoByDevice)
	volumes, err := supplementPartLabel(files, mountInfoByDevice)
	if err != nil {
		logger.Error("mount_supplement_partlabel", mounts, err)
		return
	}

	_, err = os.Stat(PathPrefix)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(PathPrefix, os.ModeDir+0755)
			if err != nil {
				logger.Error("mount_mkdir_for_mountpoint", PathPrefix, err)
				return
			}
		}
	}
	logger.Info("in_mount", volumes)
	for deviceName, devicePartInfo := range volumes {
		//mountPath used as the mountpoint which compositor with device name and label name
		mountPath := PathPrefix + filepath.Base(deviceName)
		if len(devicePartInfo.LabelName) != 0 {
			//replace the blank with _
			labelName := strings.ReplaceAll(devicePartInfo.LabelName, " ", "_")
			mountPath += "/" + labelName
		}
		_, err = os.Stat(mountPath)
		if err != nil {
			if os.IsNotExist(err) {
				err = os.Mkdir(mountPath, os.ModeDir+0755)
				if err != nil {
					logger.Error("mount_mkdir_for_volumes", devicePartInfo, err)
					return
				}
			} else {
				logger.Error("mount_stat_volume", mountPath, err)
				err = syscall.Unmount(mountPath, 0)
				if err != nil {
					logger.Error("umount_volumes", mountPath, err)
					return
				}
			}
		}

		mArgs = append(mArgs, MountArgs{
			Args: []string{"-o", "allow_other", mountPath},
			PassFS: Ptfs{
				root: devicePartInfo.MountPoint,
			},
		})
	}
	return
}

type MountArgs struct {
	Args   []string
	PassFS Ptfs
}

type volumeInfo struct {
	Volume     string
	MountPoint string // used to decide the root dir of the volume
	MountID    string //used to decide the original mounting by selecting the minimum number
	LabelName  string //used to be a elemet of formming a mount point
	// MountType  string
}

const LenFieldOfSelfMountInfo = 9
const indexDevice = 9
const indexFileType = 8
const indexPath = 3
const indexMountPoint = 4
const indexMountID = 0

func readDevicesAndMountPoint(mounts []byte) map[string]volumeInfo {
	var mountInfoByDevice map[string]volumeInfo
	mountInfoByDevice = make(map[string]volumeInfo)
	lines := strings.Split(string(mounts), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		//below is a line example of the mountinfo
		//35 29 8:5 / /data rw,relatime shared:7 - ext4 /dev/sda5 rw
		//807 790 7:1 / /var/lib/waydroid/rootfs/vendor ro,relatime shared:446 - ext4 /dev/loop1 ro
		//29 1 252:0 / / rw,relatime shared:1 - ext4 /dev/mapper/vg-root rw
		if len(fields) < LenFieldOfSelfMountInfo {
			continue
		}
		//continue if the third element is great than one char
		if len(fields[indexPath]) > 1 {
			continue
		}
		//continue if the filesystem is not ext4
		if fields[indexFileType] != "ext4" {
			continue
		}
		//continue if the device is a loop device
		if strings.Contains(fields[indexDevice], "loop") {
			continue
		}
		mountPoint := fields[indexMountPoint]
		mountID := fields[indexMountID]
		//whether a device is lvm
		if strings.Contains(fields[indexDevice], "/dev/mapper") {
			name, err := os.Readlink(fields[indexDevice])
			if err != nil {
				logger.Error("read_volumes_for_lvm", name, err)
				return nil
			}
			//name is like ../../sda should replace the name with the actually device
			name = strings.Replace(name, "..", "/dev", 1)
			fields[indexDevice] = name
		}
		if value, exist := mountInfoByDevice[fields[indexDevice]]; exist {
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
		mountInfoByDevice[fields[indexDevice]] = volumeInfo{
			MountPoint: mountPoint,
			MountID:    mountID,
		}
	}
	return mountInfoByDevice

}

func supplementPartLabel(files []fs.FileInfo, mountInfoByDevice map[string]volumeInfo) (map[string]volumeInfo, error) {
	var volumesByDevice map[string]volumeInfo
	volumesByDevice = make(map[string]volumeInfo)
	for _, v := range files {
		name, err := os.Readlink(filepath.Join("/dev/disk/by-partlabel/", v.Name()))
		if err != nil {
			logger.Error("read_volumes", name, err)
			return nil, err
		}
		//to get the name of device like /dev/sda
		name = strings.Replace(name, "../..", "/dev", 1)
		if value, exist := mountInfoByDevice[name]; exist {
			volumesByDevice[name] = volumeInfo{
				LabelName:  v.Name(),
				MountPoint: value.MountPoint,
				MountID:    value.MountID,
			}
		}
	}
	return volumesByDevice, nil
}

// func supplementVolume(files []fs.FileInfo, mountInfoByDevice map[string]volumeInfo) (map[string]volumeInfo, error) {
// 	var volumesByDevice map[string]volumeInfo
// 	volumesByDevice = make(map[string]volumeInfo)
// 	for _, v := range files {
// 		name, err := os.Readlink(filepath.Join("/dev/disk/by-uuid/", v.Name()))
// 		if err != nil {
// 			logger.Error("read_volumes", name, err)
// 			return nil, err
// 		}
// 		//to get the name of device like /dev/sda
// 		name = strings.Replace(name, "../..", "/dev", 1)
// 		if value, exist := mountInfoByDevice[name]; exist {
// 			volumesByDevice[name] = volumeInfo{
// 				Volume:     v.Name(),
// 				MountPoint: value.MountPoint,
// 				MountID:    value.MountID,
// 			}
// 		}
// 	}
// 	return volumesByDevice, nil
// }

func UmountAllVolumes() error {
	entries, err := os.ReadDir(PathPrefix)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("mount_query_home_failed", os.Getuid(), err)
		return err
	}
	openfde := filepath.Join(home, "openfde")
	syscall.Setreuid(-1, 0)
	syscall.Unmount(openfde, 0)
	for _, volume := range entries {
		path := PathPrefix + volume.Name()
		err = syscall.Unmount(path, 0)
		if err != nil {
			logger.Error("umount_volumes", path, err)
		}
	}
	return nil
}

var _version_ = "v0.1"
var _tag_ = "v0.1"
var _date_ = "20231001"

func main() {
	var umount, mount, help, version, debug bool
	flag.BoolVar(&mount, "m", false, "-m")
	flag.BoolVar(&version, "v", false, "-v")
	flag.BoolVar(&umount, "u", false, "-u")
	flag.BoolVar(&help, "h", false, "-h")
	flag.BoolVar(&debug, "d", false, "-d")
	flag.Parse()

	switch {
	case help:
		{
			fmt.Println("fde_fs:")
			fmt.Println("\t-h: help")
			fmt.Println("\t-v: print version and tag")
			fmt.Println("\t-u: umount all volumes")
			fmt.Println("\t-m: mount all volumes")
			fmt.Println("\t-d: debug mode")
			return
		}
	case version:
		{
			fmt.Printf("Version: %s, tag: %s , date: %s \n", _version_, _tag_, _date_)
			return
		}
	case umount:
		{
			logger.Info("umount_all_volumes", "umount")
			err := UmountAllVolumes()
			if err != nil {
				logger.Error("umount_failed", nil, err)
			}
			return
		}
	}

	if !mount {
		return
	}
	//mount /HOME/.local/share/openfde on /HOME/openfde
	dataOrigin, dataPoint, err := MKDataDir()
	if err != nil {
		os.Exit(1)
	}
	mountArgs, err := ConstructMountArgs()
	if err != nil {
		os.Exit(1)
	}
	//var mountArgs []MountArgs
	args := []string{"-o", "allow_other", "-o", "nonempty"}
	if debug {
		args = append(args, "-o", "debug")
	}
	args = append(args, dataPoint)
	mountArgs = append(mountArgs, MountArgs{
		Args: args,
		PassFS: Ptfs{
			root: dataOrigin,
		},
	})
	var wg sync.WaitGroup
	wg.Add(len(mountArgs))
	ch := make(chan struct{})
	hosts := make([]*fuse.FileSystemHost, len(mountArgs))
	for index, value := range mountArgs {
		go func(args []string, fs Ptfs, c chan struct{}) {
			defer wg.Done()
			hosts[index] = fuse.NewFileSystemHost(&fs)
			if debug {
				fmt.Println(args, fs.root)
			}
			tr := hosts[index].Mount("", args)
			if !tr {
				logger.Error("mount_fuse_error", tr, nil)
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
	logger.Info("mount_exit", "exit")
}
