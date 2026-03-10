package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	indexer  *Indexer
	watcher  *fsnotify.Watcher
	debounce time.Duration
	done     chan struct{}
}

func NewWatcher(idx *Indexer, debounce time.Duration) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		indexer:  idx,
		watcher:  fsWatcher,
		debounce: debounce,
		done:     make(chan struct{}),
	}

	go w.processEvents()

	return w, nil
}

func (w *Watcher) Watch(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return w.watcher.Add(path)
		}
		return nil
	})
}

func (w *Watcher) processEvents() {
	var pending map[string]bool
	var timer *time.Timer

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if !strings.HasSuffix(event.Name, ".go") {
				continue
			}
			if pending == nil {
				pending = make(map[string]bool)
			}
			pending[event.Name] = true

			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, func() {
				for path := range pending {
					w.indexer.IndexFile(path)
				}
				pending = nil
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			_ = err
		}
	}
}

func (w *Watcher) Close() {
	close(w.done)
	w.watcher.Close()
}
