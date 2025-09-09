package main

import (
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
	cmd := exec.Command("waydroid", "shell", "settings", "put", "secure", "navigation_mode", string(mode))
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil

}
