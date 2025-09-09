package main

import (
	"fde_fs/logger"
	"os"
	"syscall"
)

func createLogFile() {
	oldUmask := syscall.Umask(0)
	defer syscall.Umask(oldUmask) // 恢复原umask
	if _, err := os.Stat("/var/log"); os.IsNotExist(err) {
		err := os.MkdirAll("/var/log", 0755)
		if err != nil {
			logger.Error("create_log_dir_failed", "/var/log", err)
			return
		}
	}
	if _, err := os.Stat("/var/log/fde.log"); os.IsNotExist(err) {
		file, err := os.OpenFile("/var/log/fde.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			logger.Error("create_log_file_failed", "/var/log/fde.log", err)
			return
		}
		file.Close()
	}
	return
}

func lograteFDE() {
	oldUmask := syscall.Umask(0)
	defer syscall.Umask(oldUmask) // 恢复原umask
	if _, err := os.Stat("/var/log/fde.log"); os.IsNotExist(err) {
		logger.Error("lograte_file_not_exist", "/var/log/fde.log", err)
		return
	}
	if _, err := os.Stat("/var/log/fde.log.1"); err == nil {
		err := os.Remove("/var/log/fde.log.1")
		if err != nil {
			logger.Error("lograte_remove_old_log_failed", nil, err)
			return
		}
	}
	err := os.Rename("/var/log/fde.log", "/var/log/fde.log.1")
	if err != nil {
		logger.Error("lograte_rename_failed", nil, err)
		return
	}
	file, err := os.OpenFile("/var/log/fde.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		logger.Error("lograte_create_new_log_failed", nil, err)
		return
	}
	file.Close()
	logger.Info("lograte_sed_executed", "sed command executed successfully")
	return
}
