package main

import (
	"fde_fs/logger"
	"os/exec"
	"strings"
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
	logger.Info("set_mode", mode)
	var cmd *exec.Cmd
	if mode == Navigation3Btn {
		cmd = exec.Command("waydroid", "shell", "cmd overlay enable-exclusive  com.android.internal.systemui.navbar.threebutton")
	} else if mode == NavigationGesture {
		cmd = exec.Command("waydroid", "shell", "cmd overlay enable-exclusive  com.android.internal.systemui.navbar.gestural_extra_wide_back")
	}
	err := cmd.Run()
	if err != nil {
		logger.Error("set_mode_filed", mode, err)
		return err
	}
	return nil

}
