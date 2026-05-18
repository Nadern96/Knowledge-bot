package wiki

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Loader struct {
	repoPath string
	mu       sync.RWMutex
	context  string
}

func NewLoader(repoPath string) *Loader {
	return &Loader{repoPath: repoPath}
}

func (l *Loader) Load() error {
	entries, err := filepath.Glob(filepath.Join(l.repoPath, "*.md"))
	if err != nil {
		return err
	}

	var sb strings.Builder
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("skipping %s: %v", path, err)
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		fmt.Fprintf(&sb, "## Page: %s\n\n%s\n\n---\n\n", name, string(data))
	}

	l.mu.Lock()
	l.context = sb.String()
	l.mu.Unlock()

	log.Printf("wiki loaded: %d pages", len(entries))
	return nil
}

func (l *Loader) StartRefresh(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := l.pull(); err != nil {
				log.Printf("git pull failed: %v", err)
			}
			if err := l.Load(); err != nil {
				log.Printf("wiki reload failed: %v", err)
			}
		}
	}()
}

func (l *Loader) Context() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.context
}

func (l *Loader) pull() error {
	cmd := exec.Command("git", "-C", l.repoPath, "pull")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
