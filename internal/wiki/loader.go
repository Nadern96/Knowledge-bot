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

type Source struct {
	Name string
	Path string
}

type Loader struct {
	sources []Source
	mu      sync.RWMutex
	pages   map[string]string // source name → concatenated content
}

func NewLoader(sources []Source) *Loader {
	return &Loader{
		sources: sources,
		pages:   make(map[string]string),
	}
}

func (l *Loader) Load() error {
	for _, src := range l.sources {
		content, err := loadDir(src.Path)
		if err != nil {
			log.Printf("skipping source %q: %v", src.Name, err)
			continue
		}
		l.mu.Lock()
		l.pages[src.Name] = content
		l.mu.Unlock()
		log.Printf("source %q loaded", src.Name)
	}
	return nil
}

func (l *Loader) StartRefresh(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			for _, src := range l.sources {
				if isGitRepo(src.Path) {
					if err := gitPull(src.Path); err != nil {
						log.Printf("git pull failed for %q: %v", src.Name, err)
					}
				}
				content, err := loadDir(src.Path)
				if err != nil {
					log.Printf("reload failed for %q: %v", src.Name, err)
					continue
				}
				l.mu.Lock()
				l.pages[src.Name] = content
				l.mu.Unlock()
			}
		}
	}()
}

// Context returns the wiki content for a specific source, or all sources
// concatenated if filter is empty.
func (l *Loader) Context(filter string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if filter != "" {
		content, ok := l.pages[filter]
		if !ok {
			return fmt.Sprintf("(no source named %q found)", filter)
		}
		return content
	}

	var sb strings.Builder
	for name, content := range l.pages {
		fmt.Fprintf(&sb, "# Source: %s\n\n%s\n\n", name, content)
	}
	return sb.String()
}

// SourceNames returns the list of configured source names.
func (l *Loader) SourceNames() []string {
	names := make([]string, 0, len(l.sources))
	for _, s := range l.sources {
		names = append(names, s.Name)
	}
	return names
}

func loadDir(path string) (string, error) {
	entries, err := filepath.Glob(filepath.Join(path, "*.md"))
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no .md files found in %s", path)
	}

	var sb strings.Builder
	for _, p := range entries {
		data, err := os.ReadFile(p)
		if err != nil {
			log.Printf("skipping %s: %v", p, err)
			continue
		}
		name := strings.TrimSuffix(filepath.Base(p), ".md")
		fmt.Fprintf(&sb, "## Page: %s\n\n%s\n\n---\n\n", name, string(data))
	}
	return sb.String(), nil
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

func gitPull(path string) error {
	cmd := exec.Command("git", "-C", path, "pull")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}
