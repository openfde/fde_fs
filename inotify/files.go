package inotify

import (
	"fmt"
	"log"
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
		fmt.Println(offset)
		for offset < uint32(n) {
			event := (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			name := strings.TrimRight(string(buf[offset+unix.SizeofInotifyEvent:offset+unix.SizeofInotifyEvent+event.Len]), "\x00")
			fullPath := filepath.Join(path, name)

			if event.Mask&unix.IN_CREATE != 0 {
				//if strings.HasSuffix(name, ".desktop") {
				message := fmt.Sprintf("New desktop file created: %s", fullPath)
				addevents <- message
				//}
			} else if event.Mask&unix.IN_DELETE != 0 {
				//if strings.HasSuffix(name, ".desktop") {
				message := fmt.Sprintf("Desktop file deleted: %s", fullPath)
				delevents <- message
				//}
			}

			offset += unix.SizeofInotifyEvent + event.Len
		}
	}
}

func WatchDesktop(desktopDir string) {

	addevents := make(chan string)
	delevents := make(chan string)
	go watchDirectory(desktopDir, addevents, delevents)

	for {
		select {
		case event := <-addevents:
			fmt.Println("add", event)
		case event := <-delevents:
			fmt.Println("del", event)
		}
	}
}
