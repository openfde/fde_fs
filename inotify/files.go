package inotify

import (
	"context"
	"encoding/json"
	"fde_fs/logger"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

func watchDirectory(path string, addevents, delevents chan string) {
	fd, err := unix.InotifyInit()
	if err != nil {
		log.Fatalf("Failed to initialize inotify: %v", err)
	}
	defer unix.Close(fd)

	wd, err := unix.InotifyAddWatch(fd, path, unix.IN_CREATE|unix.IN_DELETE)
	if err != nil {
		log.Fatalf("Failed to add inotify watch: %v", err)
	}
	defer unix.InotifyRmWatch(fd, uint32(wd))

	buf := make([]byte, 4096)
	for {
		n, err := unix.Read(fd, buf)
		if err != nil {
			log.Fatalf("Failed to read inotify events: %v", err)
		}

		var offset uint32
		for offset < uint32(n) {
			event := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			name := strings.TrimRight(string(buf[offset+unix.SizeofInotifyEvent:offset+unix.SizeofInotifyEvent+event.Len]), "\x00")
			fullPath := filepath.Join(path, name)

			if event.Mask&unix.IN_CREATE != 0 {

				//if strings.HasSuffix(name, ".desktop") {
				message := fmt.Sprintf("%s", fullPath)
				addevents <- message
				//}
			} else if event.Mask&unix.IN_DELETE != 0 {
				//if strings.HasSuffix(name, ".desktop") {
				message := fmt.Sprintf("%s", fullPath)
				delevents <- message
				//}
			}

			offset += unix.SizeofInotifyEvent + event.Len
		}
	}
}

type Op string

const (
	ADD    Op = "add"
	DELETE Op = "delete"
)

type InotifyEvent struct {
	FileName string
	OpCode   Op // "add" or "delete"
}

func WatchDesktop(ctx context.Context, desktopDir string) {

	addevents := make(chan string)
	delevents := make(chan string)
	go watchDirectory(desktopDir, addevents, delevents)

	for {
		select {
		case event := <-addevents:
			{
				IEvent := InotifyEvent{
					FileName: event,
					OpCode:   ADD,
				}
				encode, err := json.Marshal(IEvent)
				if err != nil {
					logger.Error("json_marshal_error", event, err)
					continue
				}
				cmd := exec.Command("waydroid", "inotify", string(encode))
				if err := cmd.Run(); err != nil {
					logger.Error("command_execution_error", nil, err)
					continue
				}
			}

		case event := <-delevents:
			{
				IEvent := InotifyEvent{
					FileName: event,
					OpCode:   DELETE,
				}
				encode, err := json.Marshal(IEvent)
				if err != nil {
					logger.Error("json_marshal_error", event, err)
					continue
				}
				cmd := exec.Command("waydroid", "inotify", string(encode))
				if err := cmd.Run(); err != nil {
					logger.Error("command_execution_error", nil, err)
					continue
				}
			}
		case <-ctx.Done():
			{
				logger.Info("context_cancelled", "inotify received context cancel")
				return
			}
		}
	}
}
