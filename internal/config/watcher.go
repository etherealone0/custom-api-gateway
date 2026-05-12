package config

import (
	"log/slog"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type OnReloadFunc func(cfg *Config)

type Watcher struct {
	path      string
	watcher   *fsnotify.Watcher
	onReload  OnReloadFunc
	stopOnce  sync.Once
	done      chan struct{}
}

func NewWatcher(path string, onReload OnReloadFunc) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := fw.Add(path); err != nil {
		fw.Close()
		return nil, err
	}

	w := &Watcher{
		path:     path,
		watcher:  fw,
		onReload: onReload,
		done:     make(chan struct{}),
	}

	go w.loop()
	return w, nil
}

func (w *Watcher) loop() {
	defer close(w.done)

	var debounce *time.Timer

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			if !event.Has(fsnotify.Write) {
				continue
			}

			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(100*time.Millisecond, func() {
				w.reload()
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("config watcher error", "error", err)
		}
	}
}

func (w *Watcher) reload() {
	slog.Info("config file changed, reloading", "path", w.path)

	cfg, err := Load(w.path)
	if err != nil {
		slog.Error("config reload failed, keeping current config", "error", err)
		return
	}

	slog.Info("config reloaded successfully", "routes", len(cfg.Routes))
	w.onReload(cfg)
}

func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		w.watcher.Close()
		<-w.done
	})
}
