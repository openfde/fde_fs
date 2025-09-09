package main

import (
	"fde_fs/logger"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var aospVersion string
var LocalOpenfde string

func readAospVersion() {
	const codeKey = "ro.vendor.build.version.release_or_codename="
	// Mount /usr/share/waydroid-extra/images/vendor.img to /tmp
	vendorImgPath := "/usr/share/waydroid-extra/images/vendor.img"
	tmpMountPoint := "/tmp/vendor_mount"
	buildpropPath := "/var/lib/waydroid/rootfs/vendor/build.prop"
	var err error
	if _, err = os.Stat(buildpropPath); err == nil {
		// Read ro.vendor.build.version.release_or_codename from build.prop
		content, err := ioutil.ReadFile(buildpropPath)
		if err != nil {
			logger.Error("read_build_prop", buildpropPath, err)
			return
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, codeKey) {
				aospVersion = strings.TrimPrefix(line, codeKey)
				logger.Info("vendor_build_version", aospVersion)
				return
			}
		}
	}
	if _, err = os.Stat(vendorImgPath); err == nil {
		// Create mount point if it doesn't exist
		if err = os.MkdirAll(tmpMountPoint, 0755); err != nil {
			logger.Error("mkdir_vendor_mount", tmpMountPoint, err)
			return
		} else {
			// Check if already mounted
			checkCmd := exec.Command("sh", "-c", fmt.Sprintf("cat /proc/mounts | grep -w %s", tmpMountPoint))
			if output, err := checkCmd.Output(); err == nil && len(output) > 0 {
				logger.Info("vendor_mount_already_exists", string(output))
			} else {
				// Mount vendor.img
				cmd := exec.Command("mount", "-o", "loop,ro", vendorImgPath, tmpMountPoint)
				if err = cmd.Run(); err != nil {
					logger.Error("mount_vendor_img", vendorImgPath, err)
					return
				}
			}
			defer func() {
				// Unmount the vendor.img
				umountCmd := exec.Command("umount", tmpMountPoint)
				if umountErr := umountCmd.Run(); umountErr != nil {
					logger.Error("umount_vendor_img", tmpMountPoint, umountErr)
				}
			}()
			// Read ro.vendor.build.version.release_or_codename from build.prop
			buildPropPath := filepath.Join(tmpMountPoint, "build.prop")
			var content []byte
			if content, err = ioutil.ReadFile(buildPropPath); err == nil {
				lines := strings.Split(string(content), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, codeKey) {
						aospVersion = strings.TrimPrefix(line, codeKey)
						logger.Info("vendor_build_version", aospVersion)
						break
					}
				}
			} else {
				logger.Error("read_build_prop", buildPropPath, err)
				return
			}
		}
	}
	return
}
