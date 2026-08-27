package workspace

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tuttitypes "github.com/tutti-os/tutti/services/tuttid/types"
)

func workspaceAppCLIPath() (string, error) {
	return workspaceAppCLIPathForPlatform(runtime.GOOS)
}

func workspaceAppCLIPathForPlatform(platform string) (string, error) {
	if platform != "windows" {
		return resolveWorkspaceAppCLIPathUnix(platform), nil
	}

	configured := strings.TrimSpace(os.Getenv("TUTTI_WORKSPACE_APP_CLI_PATH"))
	if configured == "" {
		return "", fmt.Errorf("native Tutti CLI path is not configured")
	}
	if !filepath.IsAbs(configured) {
		return "", fmt.Errorf("native Tutti CLI path must be absolute")
	}
	if !strings.EqualFold(filepath.Ext(configured), ".exe") {
		return "", fmt.Errorf("native Tutti CLI path must point to an .exe")
	}
	info, err := os.Stat(configured)
	if err != nil {
		return "", fmt.Errorf("inspect native Tutti CLI: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("native Tutti CLI path must point to a regular file")
	}
	return configured, nil
}

// resolveWorkspaceAppCLIPathUnix picks the `tutti` CLI apps shell out to. In a
// normal desktop install the desktop app writes a shim at <stateDir>/bin/tutti;
// but a headless / remote-only daemon (no desktop) never creates it. To keep
// apps working there, resolve in order:
//  1. an explicit TUTTI_WORKSPACE_APP_CLI_PATH override,
//  2. the standard shim path if it exists,
//  3. a `tutti`/`tutti-dev` discovered on the daemon's PATH,
//  4. the shim path as a last resort (preserves prior behavior / error surface).
func resolveWorkspaceAppCLIPathUnix(platform string) string {
	if configured := strings.TrimSpace(os.Getenv("TUTTI_WORKSPACE_APP_CLI_PATH")); configured != "" {
		return configured
	}

	shimPath := tuttiCLIShimPathForPlatform(platform)
	if fileExists(shimPath) {
		return shimPath
	}

	if resolved, err := exec.LookPath(tuttiCLICommandName()); err == nil && strings.TrimSpace(resolved) != "" {
		return resolved
	}

	return shimPath
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func workspaceAppCLIEnvOverrides(platform string, cliPath string) []string {
	overrides := []string{"TUTTI_CLI=" + cliPath}
	if platform == "windows" {
		overrides = append(overrides, "TUTTID_LISTENER_INFO_PATH="+tuttitypes.TuttidListenerInfoPath())
	}
	return overrides
}
