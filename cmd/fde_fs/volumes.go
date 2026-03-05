package main

import (
	"encoding/json"
	"fde_fs/logger"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const FSPrefix = "volumes"
const VolumesPathPrefix = "/var/lib/fde/volumes/"

type uuidToPath struct {
	UUID string
	Path string
}

func ConstructMountArgs() (mArgs []MountArgs, err error) {
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

	//register the volumes info into fde_ctrl

	_, err = os.Stat(VolumesPathPrefix)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(VolumesPathPrefix, os.ModeDir+0755)
			if err != nil {
				logger.Error("mount_mkdir_for_volumes", VolumesPathPrefix, err)
				return
			}
		}
	}
	logger.Info("in_mount", volumes)
	var uuidToPaths []uuidToPath
	for _, mountInfo := range volumes {
		path := VolumesPathPrefix + mountInfo.VolumeUUID
		uuidToPaths = append(uuidToPaths, uuidToPath{
			UUID: mountInfo.VolumeUUID,
			Path: mountInfo.MountPoint,
		})
		_, err = os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				err = os.Mkdir(path, os.ModeDir+0755)
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

		mArgs = append(mArgs, MountArgs{
			Args: []string{"-o", "allow_other", VolumesPathPrefix + mountInfo.VolumeUUID},
			PassFS: Ptfs{
				root: mountInfo.MountPoint,
			},
		})
	}
	if len(uuidToPaths) > 0 {
		err := WriteJSONToFile(VolumesPathPrefix+".fde_path_key", uuidToPaths)
		if err != nil {
			logger.Error("write_fde_path", uuidToPaths, err)
		}
	}
	return
}

type volumeAndMountPoint struct {
	VolumeUUID string
	MountPoint string
	MountID    string
	// MountType  string
}

const LenFieldOfSelfMountInfo = 9
const indexDevice = 9
const indexFileType = 8
const indexPath = 3
const indexMountPoint = 4
const indexMountID = 0

func readDevicesAndMountPoint(mounts []byte) map[string]volumeAndMountPoint {
	var mountInfoByDevice map[string]volumeAndMountPoint
	mountInfoByDevice = make(map[string]volumeAndMountPoint)
	lines := strings.Split(string(mounts), "\n")
	var rootMountPointFlg = false
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
		if mountPoint == "/" {
			rootMountPointFlg = true
		}
		mountID := fields[indexMountID]
		//whether a device is lvm
		if strings.Contains(fields[indexDevice], "/dev/mapper") {
			name, err := os.Readlink(fields[indexDevice])
			if err != nil {
				logger.Error("read_volumes_for_lvm", name, err)
				return nil
			}
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
		mountInfoByDevice[fields[indexDevice]] = volumeAndMountPoint{
			MountPoint: mountPoint,
			MountID:    mountID,
		}
	}
	if !rootMountPointFlg {
		data, err := os.ReadFile("/proc/self/mounts")
		if err != nil {
			logger.Error("read_proc_mount_for_root_failed", nil, err)
			return mountInfoByDevice
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			if fields[1] == "/" {
				mountInfoByDevice[fields[0]] = volumeAndMountPoint{
					MountPoint: "/",
					MountID:    "0",
				}
				break
			}
		}
	}
	return mountInfoByDevice
}

type MountArgs struct {
	Args   []string
	PassFS Ptfs
}

func WriteJSONToFile(filename string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return err
	}

	return nil
}

func supplementVolume(files []fs.FileInfo, mountInfoByDevice map[string]volumeAndMountPoint) (map[string]volumeAndMountPoint, error) {
	var volumesByDevice map[string]volumeAndMountPoint
	volumesByDevice = make(map[string]volumeAndMountPoint)
	for _, v := range files {
		name, err := os.Readlink(filepath.Join("/dev/disk/by-uuid/", v.Name()))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			logger.Error("read_volumes", name, err)
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		name = strings.Replace(name, "../..", "/dev", 1)
		if value, exist := mountInfoByDevice[name]; exist {
			volumesByDevice[name] = volumeAndMountPoint{
				VolumeUUID: v.Name(),
				MountPoint: value.MountPoint,
				MountID:    value.MountID,
			}
		}
	}
	return volumesByDevice, nil
}

func UmountAllVolumes() error {
	entries, err := os.ReadDir(VolumesPathPrefix)
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
		if !volume.IsDir() {
			continue
		}
		path := VolumesPathPrefix + volume.Name()
		err = syscall.Unmount(path, 0)
		if err != nil {
			logger.Error("umount_volumes", path, err)
			os.Remove(path)
		}
	}
	return nil
}
