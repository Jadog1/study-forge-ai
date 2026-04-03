package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/studyforge/study-agent/internal/config"
	"github.com/studyforge/study-agent/internal/orchestrator"
	"github.com/studyforge/study-agent/internal/server"
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the web UI server",
	Long:  "Launches an HTTP server that serves the Study Forge AI web interface and API.",
	RunE:  runWeb,
}

func init() {
	webCmd.Flags().IntP("port", "p", 8080, "port to listen on")
	webCmd.Flags().Bool("no-browser", false, "do not open browser automatically")
	webCmd.Flags().Bool("dev", false, "development mode (API only, frontend served by Vite)")
	rootCmd.AddCommand(webCmd)
}

func runWeb(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	noBrowser, _ := cmd.Flags().GetBool("no-browser")
	devMode, _ := cmd.Flags().GetBool("dev")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	orch := orchestrator.NewFallback(cfg)

	staticDir := ""
	if !devMode {
		staticDir = findStaticDir()
	}

	srv := server.New(cfg, orch, port, staticDir)

	if !noBrowser {
		url := srv.Addr()
		if devMode {
			url = "http://localhost:5173"
		}
		go openBrowser(url)
	}

	if devMode {
		fmt.Printf("Study Forge AI API server (dev mode): %s\n", srv.Addr())
		fmt.Println("Run 'npm run dev' in web/ for the frontend.")
	} else if staticDir != "" {
		fmt.Printf("Study Forge AI web server: %s\n", srv.Addr())
	} else {
		fmt.Printf("Study Forge AI API server: %s\n", srv.Addr())
		fmt.Println("No built frontend found. Run 'npm run build' in web/ first, or use --dev mode.")
	}

	return srv.ListenAndServe()
}

func findStaticDir() string {
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "web", "dist")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		candidate := filepath.Join(cwd, "web", "dist")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}
