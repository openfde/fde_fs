package main

import (
	"encoding/json"
	"errors"
	"fde_fs/cmd/fde_fs/personal_fusing"
	"fde_fs/logger"
	"fde_fs/logo"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
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

	_, err = os.Stat(PathPrefix)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(PathPrefix, os.ModeDir+0755)
			if err != nil {
				logger.Error("mount_mkdir_for_volumes", PathPrefix, err)
				return
			}
		}
	}
	logger.Info("in_mount", volumes)
	var uuidToPaths []uuidToPath
	for _, mountInfo := range volumes {
		path := PathPrefix + mountInfo.VolumeUUID
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
			Args: []string{"-o", "allow_other", PathPrefix + mountInfo.VolumeUUID},
			PassFS: Ptfs{
				root: mountInfo.MountPoint,
			},
		})
	}
	if len(uuidToPaths) > 0 {
		err := WriteJSONToFile("/volumes/.fde_path_key", uuidToPaths)
		if err != nil {
			logger.Error("write_fde_path", uuidToPaths, err)
		}
	}
	return
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

type MountArgs struct {
	Args   []string
	PassFS Ptfs
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
	return mountInfoByDevice

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
		if !volume.IsDir() {
			continue
		}
		path := PathPrefix + volume.Name()
		err = syscall.Unmount(path, 0)
		if err != nil {
			logger.Error("umount_volumes", path, err)
			os.Remove(path)
		}
	}
	return nil
}

var _version_ = "v0.1"
var _tag_ = "v0.1"
var _date_ = "20231001"

func getStatus() (string, error) {
	if _, err := os.Stat("/usr/sbin/getstatus"); os.IsNotExist(err) {
		logger.Info("ptfs_mount_get_status", "getstatus is not exist")
		return "", nil
	}
	cmd := exec.Command("getstatus")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func setSoftModeDepend(status string) error {
	if len(status) == 0 {
		logger.Info("ptfs_mount_get_status", "status is empty")
		return nil
	}
	lines := strings.Split(status, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "KySec status:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				status := fields[2]
				if status == "enabled" {
					cmd := exec.Command("setstatus", "softmode")
					err := cmd.Run()
					if err != nil {
						logger.Error("ptfs_mount_set_status", nil, err)
						return err
					}
					logger.Info("ptfs_mount_set_status", "Status set to softmode")
				} else {
					logger.Info("ptfs_mount_set_softmode", "already_softmode")
				}
				break
			}
		}
	}
	return nil
}

const propfile = "/var/lib/waydroid/waydroid_base.prop"

func main() {
	var umount, mount, help, version, debug, ptfsmount, ptfsumount, ptfsquery, softmode, pwrite,
		logrotate, setNavigationMode, install, restart bool
	var navi_mode string
	flag.BoolVar(&mount, "m", false, "mount volumes")
	flag.BoolVar(&version, "v", false, "version")
	flag.BoolVar(&umount, "u", false, "umount volumes")
	flag.BoolVar(&help, "h", false, "help")
	flag.BoolVar(&debug, "d", false, "debug")
	flag.BoolVar(&ptfsmount, "pm", false, "personal fusing mount")
	flag.BoolVar(&ptfsumount, "pu", false, "personal fusing umount")
	flag.BoolVar(&ptfsquery, "pq", false, "personal fusing query")
	flag.BoolVar(&softmode, "s", false, "set soft mode for kylinos")
	flag.BoolVar(&pwrite, "pwrite", false, "pwrite for sysctl")
	flag.BoolVar(&logrotate, "logrotate", false, "log rotate for /var/log/fde.log")
	flag.BoolVar(&setNavigationMode, "setnav", false, "set navigation mode")
	flag.StringVar(&navi_mode, "navmode", "0", "navigation mode,gesture(2) or 3btn(0)")
	var installPath string
	flag.StringVar(&installPath, "path", "", "path to openfde deb file")
	flag.BoolVar(&install, "install", false, "install openfde deb")
	flag.BoolVar(&restart, "restart", false, "restart fde  after install")
	flag.Parse()

	LinuxUID = os.Getuid()
	LinuxGID = os.Getgid()

	if install {
		if len(installPath) > 0 {
			syscall.Setreuid(0, 0)
			go logo.Show()
			defer logo.Disappear()
			err := installDEB(installPath)
			if err != nil {
				logger.Error("install_deb_failed", installPath, err)
				os.Exit(1)
			}
			if restart {
				_ = syscall.Setreuid(LinuxUID, 0)
				cmd := exec.Command("fde_utils", "start")
				err = cmd.Run()
				if err != nil {
					logger.Error("fde_utils_start_failed", nil, err)
				}
			}
			os.Exit(0)
		} else {
			fmt.Println("please provide the deb file path with -path")
			os.Exit(1)
		}
	}

	if ptfsquery || ptfsmount || ptfsumount || mount || setNavigationMode {
		err := syscall.Setreuid(0, 0)
		if err != nil {
			logger.Error("setreuid_error", nil, err)
			return
		}
		if setNavigationMode {
			setMode(NavigateionMode(navi_mode))
			return
		}
		readAospVersion()
		if len(aospVersion) == 0 {
			logger.Error("read_aosp_version", nil, errors.New("aosp ver empty"))
			os.Exit(1)
		}
		if aospVersion == "11" {
			aospVersion = ""
		}
		LocalOpenfde = personal_fusing.LocalShareOpenfde + aospVersion
		_ = syscall.Setreuid(LinuxUID, 0)
	}

	switch {
	case logrotate:
		{
			logrotateFDE()
			return
		}
	case pwrite:
		{
			cmd := exec.Command("sysctl", "-p")
			err := cmd.Run()
			if err != nil {
				logger.Error("sysctl_p_failed", nil, err)
				return
			}
			logger.Info("sysctl_p_executed", "sysctl -p command executed successfully")
			return
		}
	case softmode:
		{
			status, err := getStatus()
			if err != nil {
				logger.Error("soft_mode_set", nil, err)
				return
			}
			if err := setSoftModeDepend(status); err != nil {
				return
			}
			return
		}
	case ptfsquery:
		{
			mounted, err := personal_fusing.GetPtfs(aospVersion)
			if err != nil {
				os.Exit(1)
			}
			fmt.Println(mounted)
			return
		}
	case ptfsmount:
		{
			status, err := getStatus()
			if err != nil {
				logger.Error("ptfs_mount_get_status", nil, err)
				return
			}
			if err := setSoftModeDepend(status); err != nil {
				return
			}
			personal_fusing.MountPtfs(aospVersion)
			return
		}
	case ptfsumount:
		{
			personal_fusing.UmountPtfs(aospVersion)
			return
		}
	case help:
		{
			fmt.Println("fde_fs:")
			fmt.Println("\t-h: help")
			fmt.Println("\t-v: print version and tag")
			fmt.Println("\t-u: umount all volumes")
			fmt.Println("\t-pm: mount personal fusing")
			fmt.Println("\t-pu: umount personlal fusing")
			fmt.Println("\t-u: umount all volumes")
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
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigCh
		logger.Info("sigterm_received", "umount volumes")
		if err := exec.Command("fde_fs", "-u").Run(); err != nil {
			logger.Error("sig_handler_fde_fs_u_failed", nil, err)
		}
		os.Exit(0)
	}()

	//mount /HOME/.local/share/openfde on /HOME/openfde
	dataOrigin, dataPoint, err := MKDataDir(aospVersion)
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
			logger.Info("mount_volume", fmt.Sprintln(args, fs.root))
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
