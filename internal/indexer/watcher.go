// anthropic/claude-sonnet-4-6
package indexer

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/thomassaison/mcp-code-graph/internal/graph"
	"github.com/thomassaison/mcp-code-graph/internal/vector"
)

type Watcher struct {
	indexer   *Indexer
	watcher   *fsnotify.Watcher
	debounce  time.Duration
	done      chan struct{}
	persister *graph.Persister
	vector    *vector.Store

	// reindex queue: at most 1 running + 1 pending (last wins)
	reindexMu      sync.Mutex
	reindexRunning bool
	reindexPending map[string]bool
}

func NewWatcher(idx *Indexer, debounce time.Duration, persister *graph.Persister, vec *vector.Store) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		indexer:   idx,
		watcher:   fsWatcher,
		debounce:  debounce,
		done:      make(chan struct{}),
		persister: persister,
		vector:    vec,
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
			// Capture pending snapshot synchronously to avoid data race
			// between this goroutine and the timer callback goroutine
			toIndex := pending
			pending = nil
			timer = time.AfterFunc(w.debounce, func() {
				w.scheduleReindex(toIndex)
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("file watcher error", "error", err)
		}
	}
}

// scheduleReindex enqueues files for reindexing. If a reindex is already
// running, the files are stored as pending (replacing any previous pending
// batch). If nothing is running, a new reindex goroutine is started.
func (w *Watcher) scheduleReindex(files map[string]bool) {
	w.reindexMu.Lock()
	if w.reindexRunning {
		// Replace pending with the latest batch — no need to reindex stale data.
		w.reindexPending = files
		w.reindexMu.Unlock()
		return
	}
	w.reindexRunning = true
	w.reindexMu.Unlock()

	go w.runReindex(files)
}

// runReindex executes the reindex for files, then checks for a pending batch
// and runs it too, until the queue is drained.
func (w *Watcher) runReindex(files map[string]bool) {
	for {
		for path := range files {
			// Invalidate stale embeddings before re-indexing so the
			// vector store does not serve results for deleted or
			// modified functions.
			if w.vector != nil {
				for _, node := range w.indexer.Graph().GetNodesByFile(path) {
					if err := w.vector.Delete(node.ID); err != nil {
						slog.Warn("failed to delete stale embedding", "node", node.ID, "error", err)
					}
				}
			}

			if err := w.indexer.IndexFile(path); err != nil {
				slog.Warn("failed to re-index file", "path", path, "error", err)
			}
		}

		// Persist the updated graph so incremental changes survive restarts.
		if w.persister != nil {
			if err := w.persister.Save(w.indexer.Graph()); err != nil {
				slog.Warn("failed to persist graph after watcher reindex", "error", err)
			}
		}

		// Check for a pending batch queued while we were running.
		w.reindexMu.Lock()
		if w.reindexPending != nil {
			files = w.reindexPending
			w.reindexPending = nil
			w.reindexMu.Unlock()
			continue
		}
		w.reindexRunning = false
		w.reindexMu.Unlock()
		return
	}
}

func (w *Watcher) Close() {
	close(w.done)
	_ = w.watcher.Close()
}
