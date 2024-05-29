package main

import (
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

const Openfde = ".local/share/openfde"

func MKDataDir() (data, openfde string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("mount_query_home_failed", os.Getuid(), err)
		return
	}
	origin := filepath.Join(home, Openfde)
	data = filepath.Join(origin, "/media/0")
	_, err = os.Stat(data)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(data, os.ModeDir+0751)
			if err != nil {
				logger.Error("mount_mkdir_for_user_datadir", data, err)
				return
			}
			uid := os.Getuid()
			gid := os.Getgid()
			err = chownRecursive(home, "/"+Openfde, uid, gid)
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

	openfde = filepath.Join(home, "openfde")
	_, err = os.Stat(openfde)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(openfde, os.ModeDir+0751)
			if err != nil {
				logger.Error("mount_mkdir_for_user_datadir", openfde, err)
				return
			}
			err = os.Chown(openfde, os.Getuid(), os.Getgid())
			if err != nil {
				logger.Error("mount_mkdir_for_user_home_datadir", openfde, err)
				return
			}
		} else {
			//if the dir is just not umounted
			err = syscall.Unmount(openfde, 0)
			if err != nil {
				logger.Error("umount_volumes", openfde, err)
				return
			}

		}
	}
	return
}
