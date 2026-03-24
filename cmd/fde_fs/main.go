package main

import (
	"encoding/json"
	"errors"
	"fde_fs/cmd/fde_fs/personal_fusing"
	"fde_fs/logger"
	"fde_fs/logo"
	"flag"
	"fmt"
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

var _version_ = "v0.1"
var _tag_ = "v0.1"
var _date_ = "20231001"

func main() {
	var umount, mount, help, version, debug, ptfsmount, ptfsumount, ptfsquery, softmode, pwrite,
		logrotate, setNavigationMode, install, restart bool
	var navi_mode string
	var density int
	flag.BoolVar(&mount, "m", false, "mount volumes")
	flag.BoolVar(&version, "v", false, "version")
	flag.BoolVar(&umount, "u", false, "umount volumes")
	flag.BoolVar(&help, "h", false, "help")
	flag.BoolVar(&debug, "d", false, "debug")
	flag.BoolVar(&ptfsmount, "pm", false, "personal fusing mount")
	flag.BoolVar(&ptfsumount, "pu", false, "personal fusing umount")
	flag.BoolVar(&ptfsquery, "pq", false, "personal fusing query")
	flag.BoolVar(&softmode, "s", false, "off exectl for kylinos")
	flag.BoolVar(&pwrite, "pwrite", false, "pwrite for sysctl")
	flag.BoolVar(&logrotate, "logrotate", false, "log rotate for /var/log/fde.log")
	flag.BoolVar(&setNavigationMode, "setnav", false, "set navigation mode")
	flag.StringVar(&navi_mode, "navmode", "0", "navigation mode,gesture(2) or 3btn(0)")
	var installPath string
	flag.StringVar(&installPath, "path", "", "path to openfde deb file")
	flag.BoolVar(&install, "install", false, "install openfde deb")
	flag.BoolVar(&restart, "restart", false, "restart fde  after install")
	flag.IntVar(&density, "density", 0, "set screen density (-density 160 or 256)")
	flag.Parse()

	LinuxUID = os.Getuid()
	LinuxGID = os.Getgid()

	if install {
		if len(installPath) > 0 {
			syscall.Setreuid(0, 0)
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

	if ptfsquery || ptfsmount || ptfsumount || mount || setNavigationMode || density > 0 {
		err := syscall.Setreuid(0, 0)
		if err != nil {
			logger.Error("setreuid_error", nil, err)
			return
		}
		if setNavigationMode {
			setMode(NavigateionMode(navi_mode))
			return
		}
		if density > 0 {
			setDensity(density)
			return
		}
		readAospVersion()
		if len(aospVersion) == 0 {
			logger.Error("read_aosp_version", nil, errors.New("aosp ver empty"))
			if ptfsumount {
				logger.Warn("read_aosp_version_failed", "aosp version is empty, but continue to umount ptfs")
				personal_fusing.UmountPtfs("")
			}
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
			logger.Rotate()
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
				logger.Error("exectl_off_set", nil, err)
				return
			}
			if err := setExeCtlOff(status); err != nil {
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
