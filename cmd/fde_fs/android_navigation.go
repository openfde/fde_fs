package main

import (
	"os/exec"
	"strings"
	"fde_fs/logger"
)

type NavigateionMode string

const (
	NavigationGesture NavigateionMode = "0"
	Navigation3Btn    NavigateionMode = "2"
)

func readMode() string {
	cmd := exec.Command("waydroid", "shell", "settings", "get", "secure", "navigation_mode")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func setMode(mode NavigateionMode) error {
	logger.Info("set_mode",mode)
	cmd := exec.Command("waydroid", "shell", "settings", "put", "secure", "navigation_mode", string(mode))
	err := cmd.Run()
	if err != nil {
		logger.Error("set_mode_filed",mode,err)
		return err
	}
	return nil

}
