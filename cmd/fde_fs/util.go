package main

import (
	"fde_fs/logger"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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
		if strings.HasPrefix(line, "exec control") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				status := fields[2]
				if status != "off" {
					cmd := exec.Command("setstatus", "-f", "exectl", "off", "-p")
					err := cmd.Run()
					if err != nil {
						logger.Error("ptfs_mount_set_exectl_off", nil, err)
						return err
					}
					logger.Info("ptfs_mount_set_exectl_off", "exectl set to off")
				} else {
					logger.Info("ptfs_mount_set_exectl_off", "already_off")
				}
				break
			}
		}
	}
	return nil
}

func setDensity(density int) {
	if density < 120 || density > 640 {
		fmt.Println("error: a reasonable density value is typically between 120 and 640.")
		logger.Warn("error: a reasonable density value is typically between 120 and 640", nil)
		return
	}
	cmd := exec.Command("waydroid", "shell", "wm", "density", strconv.Itoa(density))
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("waydroid set density failed", map[string]interface{}{
			"density": density,
			"output":  string(output),
		}, err)
		fmt.Printf("set density failed：%v\n输出：%s\n", err, string(output))
	} else {
		logger.Warn("waydroid set density %d success\n", density)
		fmt.Printf("set density %d success\n", density)
	}
	if len(output) > 0 {
		fmt.Println("cmd output：", string(output))
	}
}
