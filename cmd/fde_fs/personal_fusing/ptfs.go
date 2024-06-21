package personal_fusing

import (
	"fde_fs/logger"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const Media0 = ".local/share/openfde/media/0/"

func UmountPtfs() error {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("mount_query_home_failed", os.Getuid(), err)
		return err
	}
	androidDir := filepath.Join(home, Media0)
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

func MountPtfs() error {
	syscall.Umask(0)

	rlinuxList, randroidList, err := getUserFolders()
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

	for i, _ := range randroidList {
		go func(source, target string) {
			defer wg.Done()
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
