package server

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var supportedIngestExts = map[string]bool{
	".md":  true,
	".txt": true,
	".rst": true,
}

type browseEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

type browseResponse struct {
	Dir     string        `json:"dir"`
	Entries []browseEntry `json:"entries"`
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	dir := r.URL.Query().Get("dir")
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "cannot determine cwd")
			return
		}
		dir = cwd
	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid path: "+err.Error())
		return
	}

	info, err := os.Stat(dir)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "cannot access: "+err.Error())
		return
	}
	if !info.IsDir() {
		jsonError(w, http.StatusBadRequest, "not a directory")
		return
	}

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "read dir: "+err.Error())
		return
	}

	var entries []browseEntry

	if dir != "/" {
		entries = append(entries, browseEntry{
			Name:  "..",
			Path:  filepath.Dir(dir),
			IsDir: true,
		})
	}

	var dirs, files []browseEntry
	for _, de := range dirEntries {
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}
		absPath := filepath.Join(dir, de.Name())
		if de.IsDir() {
			dirs = append(dirs, browseEntry{Name: de.Name(), Path: absPath, IsDir: true})
		} else if supportedIngestExts[filepath.Ext(de.Name())] {
			files = append(files, browseEntry{Name: de.Name(), Path: absPath, IsDir: false})
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	entries = append(entries, dirs...)
	entries = append(entries, files...)

	jsonResponse(w, http.StatusOK, browseResponse{Dir: dir, Entries: entries})
}
