package main

import (
	"fde_fs/logger"
	"os"
	"os/user"
	"syscall"
)

const dataDIRPrefix = "/.fde"

func MKDataDir() (dataorigin, data string, err error) {
	_, err = os.Stat(dataDIRPrefix)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(dataDIRPrefix, os.ModeDir+0777)
			if err != nil {
				logger.Error("mount_mkdir_for_datadir", dataDIRPrefix, err)
				return
			}
		}
	}
	user, err := user.Current()
	if err != nil {
		logger.Error("mount_mkdir_for_userdir", nil, err)
		return
	}
	dataorigin = dataDIRPrefix+ "/" + user.Username
	_, err = os.Stat(dataorigin)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(dataorigin, os.ModeDir+0700)
			if err != nil {
				logger.Error("mount_mkdir_for_user_datadir", dataorigin, err)
				return
			}
		}
	}
	err = os.Chown(dataorigin, os.Getuid(), os.Getgid())
	if err != nil {
		logger.Error("chown_for_home", dataorigin, err)
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("mount_query_home_failed", os.Getuid(), err)
		return
	}
	data = home + "/fde"
	_, err = os.Stat(data)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(data, os.ModeDir+0700)
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
