package inotify

import (
	"context"
	"encoding/json"
	"fde_fs/logger"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
)

func watchDirectory(path, transferedPrefix, fileType string, addevents, delevents chan string) {
	fd, err := unix.InotifyInit()
	if err != nil {
		log.Fatalf("Failed to initialize inotify: %v", err)
	}
	defer unix.Close(fd)
	if transferedPrefix == "" {
		transferedPrefix = path
	}
	wd, err := unix.InotifyAddWatch(fd, path, unix.IN_CREATE|unix.IN_MOVED_TO|unix.IN_DELETE|unix.IN_MOVED_FROM)
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
			fullPath := filepath.Join(transferedPrefix, name)
			if fileType != AnyFileType {
				if !strings.HasSuffix(name, fileType) {
					continue
				}
			}

			if event.Mask&unix.IN_CREATE != 0 || event.Mask&unix.IN_MOVED_TO != 0 {
				message := fmt.Sprintf("%s", fullPath)
				addevents <- message
			} else if event.Mask&unix.IN_MOVED_FROM != 0 || event.Mask&unix.IN_DELETE != 0 {
				message := fmt.Sprintf("%s", fullPath)
				delevents <- message
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

const ApplicationNotifyType = "application"
const AnyFileNotifyType = "desktop"

const DesktopFileType = ".desktop"
const AnyFileType = "*"

func WatchDir(ctx context.Context, dir, transferdPrefix, notifyType, fileType string) {

	addevents := make(chan string)
	delevents := make(chan string)
	go watchDirectory(dir, transferdPrefix, fileType, addevents, delevents)
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

				cmd := exec.Command("waydroid", "notify", notifyType, string(encode))
				if err := cmd.Run(); err != nil {
					logger.Error("command_execution_error", string(encode), err)
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
				cmd := exec.Command("waydroid", "notify", notifyType, string(encode))
				if err := cmd.Run(); err != nil {
					logger.Error("command_execution_error", string(encode), err)
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

func WatchDirRecursive(ctx context.Context, root,rootPrefix, notifyType string) error {
	// recursive inotify watcher implemented as a local function and used below.
	addevents := make(chan string)
	delevents := make(chan string)
	// // watchRecursive watches root and all its subdirectories. rootPrefix is prefixed to
	// // reported paths when sending on addevents/delevents. ctx cancels the whole watcher.
	// var watchRecursive func(ctx context.Context, rootPrefix, root string, addevents, delevents chan string) error
	// watchRecursive =
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	// ensure watcher closed when function returns
	go func() {
		<-ctx.Done()
		_ = watcher.Close()
	}()

	var mu sync.Mutex
	watched := make(map[string]struct{})

	// addDir registers watcher for dir and all subdirectories (recursively).
	addDir := func(dir string) error {
		return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				// ignore paths we can't access
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			mu.Lock()
			_, ok := watched[path]
			mu.Unlock()
			if ok {
				return nil
			}
			if err := watcher.Add(path); err != nil {
				// ignore failures to add individual dirs
				return nil
			}
			mu.Lock()
			watched[path] = struct{}{}
			mu.Unlock()
			return nil
		})
	}

	// initialize by adding root recursively
	if err := addDir(root); err != nil {
		_ = watcher.Close()
		return err
	}

	logger.Info("watch_dir_recursive",root)
	// event processing goroutine
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					// channel closed
					return
				}
				// prepare reported path with rootPrefix
				// remove the 'root' prefix from event.Name to get a relative path
				relPath := strings.TrimPrefix(event.Name, root)
				reportPath := filepath.Join(rootPrefix, relPath)
				logger.Info("event name :", event.Name)

				// CREATE: if directory, add watchers recursively
				if event.Op&fsnotify.Create == fsnotify.Create {
					fi, err := os.Lstat(event.Name)
					if err == nil && fi.IsDir() {
						_ = addDir(event.Name)
					}
					// send create event (file or dir)
					select {
					case addevents <- reportPath:

					case <-ctx.Done():
						return
					}
				}

				// REMOVE or RENAME: treat as deletion/move away
				if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
					// remove any watched subdirectories under this path
					mu.Lock()
					for p := range watched {
						if p == event.Name || strings.HasPrefix(p, event.Name+string(os.PathSeparator)) {
							_ = watcher.Remove(p)
							delete(watched, p)
						}
					}
					mu.Unlock()
					select {
					case delevents <- reportPath:

					case <-ctx.Done():
						return
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				_ = err // optionally log errors
			case <-ctx.Done():
				return
			}
		}
	}()

	// print events until context cancelled
	go func() {
		for {
			select {
			case p, ok := <-addevents:
				if !ok {
					return
				}
				{
					IEvent := InotifyEvent{
						FileName: p,
						OpCode:   ADD,
					}
					encode, err := json.Marshal(IEvent)
					if err != nil {
						logger.Error("json_marshal_error", p, err)
						continue
					}
					cmd := exec.Command("waydroid", "notify", notifyType, string(encode))
					if err := cmd.Run(); err != nil {
						logger.Error("command_execution_error", string(encode), err)
						continue
					}
				}
			case p, ok := <-delevents:
				if !ok {
					return
				}
				{
					IEvent := InotifyEvent{
						FileName: p,
						OpCode:   DELETE,
					}
					encode, err := json.Marshal(IEvent)
					if err != nil {
						logger.Error("json_marshal_error", p, err)
						continue
					}

					cmd := exec.Command("waydroid", "notify", notifyType, string(encode))
					if err := cmd.Run(); err != nil {
						logger.Error("command_execution_error", string(encode), err)
						continue
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// block until ctx done; caller may cancel to stop everything
	<-ctx.Done()
	// cleanup
	mu.Lock()
	for p := range watched {
		_ = watcher.Remove(p)
	}
	mu.Unlock()
	_ = watcher.Close()
	return nil
}
