package main

import (
	"fde_fs/cmd/fde_fs/personal_fusing"
	"fde_fs/logger"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var media_rw = 1023 //the uid ,1023 is media_rw of android

func chownRecursive(startPath, lastPath string, uid, gid int) error {
	dirList := strings.Split(lastPath, "/")
	for i := 0; i < len(dirList); i++ {
		if "" == dirList[i] {
			continue
		}
		startPath = filepath.Join(startPath, dirList[i])
		err := os.Chown(startPath, uid, gid)
		if err != nil {
			return err
		}
	}
	return nil
}

func MKDataDir(aospVer string) (media0, homeOpenfde string, err error) {
	localShareOpenfde := personal_fusing.LocalShareOpenfde + aospVer
	logger.Info("print_local_openfde", personal_fusing.LocalShareOpenfde)
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("mount_query_home_failed", os.Getuid(), err)
		return
	}
	origin := filepath.Join(home, localShareOpenfde)
	media0 = filepath.Join(origin, "/media/0")
	_, err = os.Stat(media0)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(media0, os.ModeDir+0751)
			if err != nil {
				logger.Error("mount_mkdir_for_user_datadir", media0, err)
				return
			}
			uid := os.Getuid()
			gid := os.Getgid()
			err = chownRecursive(home, "/"+localShareOpenfde, uid, gid)
			if err != nil {
				logger.Error("fs_chown", "openfde", err)
				return
			}
			chownRecursive(origin, "/media/0", media_rw, media_rw)
			if err != nil {
				logger.Error("fs_chown", "media_0", err)
				return
			}
		}
	}

	homeOpenfde = filepath.Join(home, "openfde")
	_, err = os.Stat(homeOpenfde)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(homeOpenfde, os.ModeDir+0751)
			if err != nil {
				logger.Error("mount_mkdir_for_user_datadir", homeOpenfde, err)
				return
			}
			err = os.Chown(homeOpenfde, os.Getuid(), os.Getgid())
			if err != nil {
				logger.Error("mount_mkdir_for_user_home_datadir", homeOpenfde, err)
				return
			}
		} else {
			//if the dir is just not umounted ,then umount it
			err = syscall.Unmount(homeOpenfde, 0)
			if err != nil {
				logger.Error("umount_volumes", homeOpenfde, err)
				return
			}

		}
	}
	return
}
