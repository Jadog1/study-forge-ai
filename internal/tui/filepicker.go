package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// supportedPickerExts mirrors the ingestion package's supported extension set.
var supportedPickerExts = map[string]bool{
	".md":  true,
	".txt": true,
	".rst": true,
}

// filePickerEntry is one row in the file picker list.
type filePickerEntry struct {
	name  string // display name
	path  string // absolute path
	isDir bool
}

// FilePickerModel is a keyboard-driven file browser overlay for multi-selecting
// individual files. It is embedded inside WorkflowModel and rendered in place
// of the normal workflow content when visible.
type FilePickerModel struct {
	visible    bool
	currentDir string
	entries    []filePickerEntry
	cursor     int
	selected   map[string]bool // absolute paths of toggled files
	viewOffset int             // first visible row
	viewRows   int             // viewport height
	statusMsg  string
	// Result state set when the picker closes.
	confirmed bool
	cancelled bool
}

func newFilePicker() FilePickerModel {
	return FilePickerModel{
		selected: make(map[string]bool),
	}
}

// Open initialises the picker at startDir (falls back to cwd).
func (fp FilePickerModel) Open(startDir string) FilePickerModel {
	fp.visible = true
	fp.confirmed = false
	fp.cancelled = false
	fp.selected = make(map[string]bool)
	fp.cursor = 0
	fp.viewOffset = 0

	dir := strings.TrimSpace(startDir)
	if dir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			dir = cwd
		}
	}

	// If startDir is a file, open its parent.
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	fp.currentDir = abs
	fp.entries = fp.loadEntries(abs)
	return fp
}

// Visible reports whether the picker is currently shown.
func (fp FilePickerModel) Visible() bool { return fp.visible }

// Done reports whether the picker has been confirmed.
func (fp FilePickerModel) Done() bool { return fp.confirmed }

// Cancelled reports whether the picker was cancelled.
func (fp FilePickerModel) Cancelled() bool { return fp.cancelled }

// SelectedFiles returns the sorted list of selected absolute file paths.
func (fp FilePickerModel) SelectedFiles() []string {
	var out []string
	for path := range fp.selected {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func (fp FilePickerModel) loadEntries(dir string) []filePickerEntry {
	dirEntries, err := os.ReadDir(dir)
	var entries []filePickerEntry

	// Always add a ".." entry unless we are at the filesystem root.
	parent := filepath.Dir(dir)
	if parent != dir {
		entries = append(entries, filePickerEntry{name: "..", path: parent, isDir: true})
	}

	if err != nil {
		return entries
	}

	// Dirs first, then supported files, both sorted by name.
	var dirs []filePickerEntry
	var files []filePickerEntry
	for _, de := range dirEntries {
		// Skip hidden entries.
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}
		abs := filepath.Join(dir, de.Name())
		if de.IsDir() {
			dirs = append(dirs, filePickerEntry{name: de.Name() + "/", path: abs, isDir: true})
		} else if supportedPickerExts[strings.ToLower(filepath.Ext(de.Name()))] {
			files = append(files, filePickerEntry{name: de.Name(), path: abs, isDir: false})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return append(append(entries, dirs...), files...)
}

func (fp FilePickerModel) current() (filePickerEntry, bool) {
	if len(fp.entries) == 0 || fp.cursor < 0 || fp.cursor >= len(fp.entries) {
		return filePickerEntry{}, false
	}
	return fp.entries[fp.cursor], true
}

// Update handles key events for the file picker.
// Returns (updated model, tea.Cmd).
func (fp FilePickerModel) Update(msg tea.Msg) (FilePickerModel, tea.Cmd) {
	if !fp.visible {
		return fp, nil
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return fp, nil
	}

	switch k.String() {
	case "esc":
		fp.visible = false
		fp.cancelled = true
		return fp, nil

	case "o", "ctrl+enter":
		fp.visible = false
		fp.confirmed = true
		return fp, nil

	case "up", "k":
		if fp.cursor > 0 {
			fp.cursor--
			fp.clampView()
		}

	case "down", "j":
		if fp.cursor < len(fp.entries)-1 {
			fp.cursor++
			fp.clampView()
		}

	case "pgup", "b":
		fp.cursor -= fp.viewRows
		if fp.cursor < 0 {
			fp.cursor = 0
		}
		fp.clampView()

	case "pgdown", "f":
		fp.cursor += fp.viewRows
		if fp.cursor >= len(fp.entries) {
			fp.cursor = len(fp.entries) - 1
		}
		fp.clampView()

	case "home", "g":
		fp.cursor = 0
		fp.clampView()

	case "end", "G":
		if len(fp.entries) > 0 {
			fp.cursor = len(fp.entries) - 1
		}
		fp.clampView()

	case "enter", " ":
		if entry, ok := fp.current(); ok {
			if entry.isDir {
				fp.currentDir = entry.path
				fp.entries = fp.loadEntries(entry.path)
				fp.cursor = 0
				fp.viewOffset = 0
			} else {
				// Toggle selection.
				if fp.selected[entry.path] {
					delete(fp.selected, entry.path)
				} else {
					fp.selected[entry.path] = true
				}
			}
		}

	case "backspace", "left", "h":
		parent := filepath.Dir(fp.currentDir)
		if parent != fp.currentDir {
			fp.currentDir = parent
			fp.entries = fp.loadEntries(parent)
			fp.cursor = 0
			fp.viewOffset = 0
		}

	case "ctrl+a":
		// Toggle: if all files in current dir are selected, deselect all; else select all.
		fileEntries := fp.fileEntriesInCurrentDir()
		allSelected := len(fileEntries) > 0
		for _, e := range fileEntries {
			if !fp.selected[e.path] {
				allSelected = false
				break
			}
		}
		for _, e := range fileEntries {
			if allSelected {
				delete(fp.selected, e.path)
			} else {
				fp.selected[e.path] = true
			}
		}
	}

	fp.statusMsg = ""
	return fp, nil
}

func (fp FilePickerModel) fileEntriesInCurrentDir() []filePickerEntry {
	var out []filePickerEntry
	for _, e := range fp.entries {
		if !e.isDir {
			out = append(out, e)
		}
	}
	return out
}

func (fp *FilePickerModel) clampView() {
	if fp.viewRows <= 0 {
		return
	}
	if fp.cursor < fp.viewOffset {
		fp.viewOffset = fp.cursor
	}
	if fp.cursor >= fp.viewOffset+fp.viewRows {
		fp.viewOffset = fp.cursor - fp.viewRows + 1
	}
}

// View renders the file picker content, sized to innerWidth × innerHeight.
func (fp *FilePickerModel) View(innerWidth, innerHeight int) string {
	// Reserve rows for header, path line, footer hint — leaves the rest for list.
	listRows := clamp(innerHeight-5, 2, innerHeight-4)
	fp.viewRows = listRows
	fp.clampView()

	var b strings.Builder
	b.WriteString(headerStyle.Render("Browse Files") + "\n")
	b.WriteString(dimStyle.Render(truncateWidth(fp.currentDir, innerWidth-2)) + "\n\n")

	// List entries.
	for i := fp.viewOffset; i < fp.viewOffset+listRows && i < len(fp.entries); i++ {
		e := fp.entries[i]
		cursor := "  "
		if i == fp.cursor {
			cursor = "▶ "
		}
		var icon string
		var nameStyle lipgloss.Style
		if e.isDir {
			icon = "[/]"
			nameStyle = labelStyle
		} else if fp.selected[e.path] {
			icon = "[✓]"
			nameStyle = selectedStyle
		} else {
			icon = "[ ]"
			nameStyle = dimStyle
		}
		row := cursor + icon + " " + e.name
		if i == fp.cursor {
			b.WriteString(warnStyle.Render(row) + "\n")
		} else {
			b.WriteString(nameStyle.Render(row) + "\n")
		}
	}

	// Padding to fill remaining rows.
	rendered := fp.viewOffset + listRows
	for i := rendered; i < fp.viewOffset+listRows; i++ {
		b.WriteString("\n")
	}

	// Status line: selection count.
	selCount := len(fp.selected)
	selStr := fmt.Sprintf("%d selected", selCount)
	if selCount == 0 {
		selStr = "no files selected"
	}
	b.WriteString("\n" + dimStyle.Render(selStr))

	// Footer hints.
	b.WriteString("\n" + dimStyle.Render("↑/↓ navigate  •  Enter/Space toggle/cd  •  ← backspace up"))
	b.WriteString("\n" + dimStyle.Render("Ctrl+A all  •  o confirm  •  Esc cancel"))

	return b.String()
}
