package main

import (
	"fde_fs/logger"
	"os"
	"syscall"
)


func MKDataDir() (dataorigin, data string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("mount_query_home_failed", os.Getuid(), err)
		return
	}
	dataorigin = home+"/.local/share/waydroid/data"
	origin := home+"/.local/share/waydroid"
	_, err = os.Stat(dataorigin)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dataorigin, os.ModeDir+0771)
			if err != nil {
				logger.Error("mount_mkdir_for_user_datadir", dataorigin, err)
				return
			}
		}
	}
	uid := os.Getuid()
	gid := os.Getgid()
	err = os.Chown(home+"/.local", uid, gid)
	err = os.Chown(home+"/.local/share", uid, gid)
	err = os.Chown(origin, uid, gid)
	if err != nil {
		logger.Error("chown_for_origin", origin, err)
		return
	}
	err = os.Chown(dataorigin, uid, gid)
	if err != nil {
		logger.Error("chown_for_dataorigin", dataorigin, err)
		return
	}
	data = home + "/openfde"
	_, err = os.Stat(data)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(data, os.ModeDir+0771)
			if err != nil {
				logger.Error("mount_mkdir_for_user_datadir", data, err)
				return
			}
			err = os.Chown(data, os.Getuid(), os.Getgid())
			if err != nil {
				logger.Error("mount_mkdir_for_user_home_datadir", data, err)
				return
			}
		}else{
		//if the dir is just not umounted
			err = syscall.Unmount(data, 0)
			if err != nil {
				logger.Error("umount_volumes",data, err)
				return
			}

		}
	}
	return
}
