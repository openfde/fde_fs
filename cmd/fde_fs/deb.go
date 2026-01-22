package main

import "os/exec"

func installDEB(debPath string) error {
	cmd := exec.Command("dpkg", "-i", debPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
