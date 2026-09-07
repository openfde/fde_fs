package main

import (
	"fde_fs/logger"
	"os/exec"
	"regexp"
	"strings"
)

var aospVersion string

func readAospVersion() {
	checkOpenfdeCmd := exec.Command("sh", "-c", "grep -A 3 waydroid_data /usr/lib/waydroid/tools/config/__init__.py | grep openfde")
	if output, err := checkOpenfdeCmd.Output(); err == nil {
		matched := regexp.MustCompile(`openfde(\d*)`).FindStringSubmatch(string(output))
		if len(matched) > 0 {
			if len(matched) > 1 {
				aospVersion = matched[1]
				if len(LocalOpenfde) == 0 {
					aospVersion = "11"
				}
			}
			logger.Info("local_openfde ", matched[0], aospVersion)
		}
	} else {
		logger.Error("read_local_openfde", "/usr/lib/waydroid/tools/config/__init__.py", err)
	}
	return

}
