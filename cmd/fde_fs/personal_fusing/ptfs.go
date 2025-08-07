package personal_fusing

import (
	"context"
	"fde_fs/inotify"
	"fde_fs/logger"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const Media0 = "/media/0/"
const LocalShareOpenfde = ".local/share/openfde"

func UmountPtfs(aospVer string) error {
	localShareOpenfde := LocalShareOpenfde + aospVer
	localMedia0 := filepath.Join(localShareOpenfde, Media0)
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("mount_query_home_failed", os.Getuid(), err)
		return err
	}
	androidDir := filepath.Join(home, filepath.Join(localMedia0))
	syscall.Setreuid(-1, 0)
	for _, dir := range androidDirList {
		logger.Info("umount_volumes", filepath.Join(androidDir, dir))
		err = syscall.Unmount(filepath.Join(androidDir, dir), 0)
		if err != nil {
			logger.Error("umount_volumes", filepath.Join(androidDir, dir), err)
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
	homeDirNameMap["Desktop"] = "桌面"
	linuxDirList = append(linuxDirList, "Documents")
	linuxDirList = append(linuxDirList, "Downloads")
	linuxDirList = append(linuxDirList, "Music")
	linuxDirList = append(linuxDirList, "Videos")
	linuxDirList = append(linuxDirList, "Pictures")
	linuxDirList = append(linuxDirList, "Desktop")

	androidDirList = append(androidDirList, "Documents")
	androidDirList = append(androidDirList, "Download")
	androidDirList = append(androidDirList, "Music")
	androidDirList = append(androidDirList, "Movies")
	androidDirList = append(androidDirList, "Pictures")
	androidDirList = append(androidDirList, "Desktop")
}

func getUserFolders(aospVer string) ([]string, []string, error) {
	localMedia0 := filepath.Join(LocalShareOpenfde+aospVer, Media0)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}
	var realLinuxDirList = make([]string, len(linuxDirList))
	var realAndroidList = make([]string, len(androidDirList))
	existEnCount := 0
	existZhCount := 0
	//stat the count of home personal dir in en and zh
	for _, v := range linuxDirList {
		_, err = os.Stat(filepath.Join(homeDir, v))
		if err == nil {
			existEnCount++
		}
		_, err = os.Stat(filepath.Join(homeDir, homeDirNameMap[v]))
		if err == nil {
			existZhCount++
		}
	}

	for i, v := range linuxDirList {
		if existEnCount <= existZhCount { //zh
			realLinuxDirList[i] = filepath.Join(homeDir, homeDirNameMap[v])
		} else { //en
			realLinuxDirList[i] = filepath.Join(homeDir, v)
		}
		realAndroidList[i] = filepath.Join(homeDir, localMedia0, androidDirList[i])
		if _, err = os.Stat(realLinuxDirList[i]); err != nil {
			if os.IsNotExist(err) {
				err = os.Mkdir(realLinuxDirList[i], os.ModeDir+0755)
				if err != nil {
					logger.Error("mkdir_personal_dir", realLinuxDirList[i], err)
					return nil, nil, err
				}
				err = os.Chown(realLinuxDirList[i], os.Getuid(), os.Getgid())
				if err != nil {
					logger.Error("chown_personal_dir", realLinuxDirList[i], err)
					return nil, nil, err
				}
			}
		}
	}
	return realLinuxDirList, realAndroidList, nil
}

func mountFdePtfs(sourcePath, targetPath string) error {
	cmd := exec.Command("fde_ptfs", "-o", "nonempty", "-o", "allow_other", sourcePath, targetPath)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func ExecuteWaydroidShell(command string) (string, error) {
	cmd := exec.Command("waydroid", "shell")
	cmd.Stdin = strings.NewReader(command)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func queryPassThroughInWaydroid() bool {
	command := "cat /proc/mounts |grep /mnt/pass_through"
	output, err := ExecuteWaydroidShell(command)
	if err != nil {
		logger.Error("execute_waydroid_shell", nil, err)
		return false
	}
	if len(output) > 0 {
		return true
	}
	return false
}

var fslock sync.Mutex

const ptfsQueryName = "fuse.fde_ptfs"

func GetPtfs(aospVer string) (bool, error) {
	_, randroidList, err := getUserFolders(aospVer)
	if err != nil {
		logger.Error("get_ptfs_get_user_forlders", nil, err)
		return false, err
	}
	mounted, _, err := getPtfs(len(randroidList))
	if err != nil {
		logger.Error("get_ptfs_query_proc", nil, err)
		return false, err
	}
	return mounted, nil
}

func getPtfs(ptfsCount int) (bool, int, error) {
	fslock.Lock()
	// Check if /proc/self/mounts contains "fde_ptfs" keyword
	mounts, err := ioutil.ReadFile("/proc/self/mounts")
	defer fslock.Unlock()
	if err != nil {
		logger.Error("read_mounts_file", nil, err)
		return false, 0, nil
	}
	ptfsActualCount := strings.Count(string(mounts), ptfsQueryName)
	if ptfsActualCount >= ptfsCount {
		logger.Info("count_ptfs", "more than "+fmt.Sprint(ptfsCount))
		return true, ptfsActualCount, nil
	} else {
		logger.Info("count_ptfs", "actualy is "+fmt.Sprint(strings.Count(string(mounts), ptfsQueryName)))
		return false, ptfsActualCount, nil
	}
}

const applicationsDir = "/usr/share/applications"

func MountPtfs(aospVer string) error {

	rlinuxList, randroidList, err := getUserFolders(aospVer)
	if err != nil {
		logger.Error("mount_dir_fusing", nil, err)
		return err
	}

	err = syscall.Setreuid(0, 0)
	if err != nil {
		logger.Error("mount_setreuid_error", nil, err)
		return err
	}
	passThroughChan := make(chan struct{})
	go func() {
		timeout := time.After(10 * time.Second) // Set a timeout of 10 seconds
		for !queryPassThroughInWaydroid() {
			logger.Info("query_pass_through", "not mounted")
			select {
			case <-timeout:
				break // Stop the loop when timeout is reached
			default:
				time.Sleep(time.Second) // Add a delay before each query
			}
		}
		passThroughChan <- struct{}{}
	}()
	select {
	case <-passThroughChan:
	}
	dirsExistChan := make(chan struct{})
	go func() {
		allExist := false
		timeout := time.After(5 * time.Second) // Set a timeout of 10 seconds
		for {
			for _, dir := range randroidList {
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					logger.Info("query_dir_exist_not", dir)
					allExist = false
				} else {
					allExist = true
				}
			}
			if allExist {
				break
			}
			select {
			case <-timeout:
				break // Stop the loop when timeout is reached
			default:
				time.Sleep(time.Second)
			}
		}
		dirsExistChan <- struct{}{}
	}()
	select {
	case <-dirsExistChan:
	}
	var wg sync.WaitGroup
	wg.Add(len(randroidList))
	ch := make(chan struct{})

	mounted, ptCount, err := getPtfs(len(randroidList))
	if err != nil {
		logger.Error("get_ptfs_error", nil, err)
		return err
	}
	if mounted {
		//already mounted
		return nil
	} else {
		if ptCount > 0 {
			UmountPtfs(aospVer) //umount first, in order to avoid only some(not all) dirs mounted
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go inotify.WatchDir(ctx, applicationsDir, inotify.ApplicationType)

	for i, _ := range randroidList {
		go func(source, target string) {
			defer wg.Done()
			if strings.Contains(target, "Desktop") {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("goroutine_panic_recovered", r, nil)
					}
				}()
				go inotify.WatchDir(ctx, source, inotify.DesktopType)
			}

			err := mountFdePtfs(source, target)
			if err != nil {
				logger.Error("mount_ptfsfuse_error", err, nil)
				ch <- struct{}{}
			}
		}(rlinuxList[i], randroidList[i])
	}

	go func() {
		wg.Wait()        //waitting for all goroutine
		ch <- struct{}{} //unlock the main goroutine
	}()
	<-ch //block here
	logger.Info("mount_ptfs_exit", "exit")
	return nil
}
