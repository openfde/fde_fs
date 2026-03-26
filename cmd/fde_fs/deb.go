package main

import (
	"bufio"
	"context"
	"fde_fs/logger"
	"os/exec"
)

func installDEB(debPath string) error {
	var cmd *exec.Cmd
	mainCtx := context.Background()
	cmd = exec.CommandContext(mainCtx, "dpkg", "-i", debPath)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	outerr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		logger.Error("dpkg_install_start", nil, err)
		return err
	}

	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		line := scanner.Text()
		logger.Error("install_deb_stdout", line, err)
	}
	scanner = bufio.NewScanner(outerr)
	for scanner.Scan() {
		line := scanner.Text()
		logger.Error("install_deb_stderr", line, err)
	}

	if err := cmd.Wait(); err != nil {
		logger.Error("dpkg_install_wait", nil, err)
		return err
	}
	return nil
}
