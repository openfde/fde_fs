package personal_fusing

import (
	"fde_fs/logger"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
)

const PathPrefix = ".local/share/openfde/media/0/"

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

func MountPtfs() error {
	syscall.Umask(0)
	rlinuxList, randroidList, err := getUserFolders()
	if err != nil {
		logger.Error("mount_dir_fusing", nil, err)
		return err
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
