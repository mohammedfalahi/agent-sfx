package setup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// UserAppDir returns the canonical, stable user-owned application directory
// independent of any transient git clone or working directory.
func UserAppDir() (string, error) {
	if custom := os.Getenv("AGENT_SFX_USER_APP_DIR"); custom != "" {
		return filepath.Clean(custom), nil
	}

	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "agent-sfx"), nil
		}
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine user config directory: %w", err)
	}
	return filepath.Join(configDir, "agent-sfx"), nil
}

// DeploymentReport summarizes the files and directories deployed to the user location.
type DeploymentReport struct {
	BaseDir            string   `json:"base_dir"`
	GeminiExtensionDir string   `json:"gemini_extension_dir"`
	ClaudePluginDir    string   `json:"claude_plugin_dir"`
	DeployedFiles      []string `json:"deployed_files"`
	PreservedFiles     []string `json:"preserved_files"`
	DryRun             bool     `json:"dry_run"`
}

// CopyFileSafely copies src to dst, preserving file permissions.
func CopyFileSafely(src, dst string, dryRun bool) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source %s: %w", src, err)
	}

	if dryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", dst, err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, srcInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create destination %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}

	return nil
}

// CopyDirRecursive copies all files and subdirectories from srcDir to dstDir.
// It preserves existing files in dstDir if overwrite is false.
func CopyDirRecursive(srcDir, dstDir string, overwrite bool, dryRun bool, report *DeploymentReport) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", srcDir, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		if entry.IsDir() {
			// Skip node_modules or transient git items if present
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
				continue
			}
			if err := CopyDirRecursive(srcPath, dstPath, overwrite, dryRun, report); err != nil {
				return err
			}
		} else {
			if _, err := os.Stat(dstPath); err == nil && !overwrite {
				report.PreservedFiles = append(report.PreservedFiles, dstPath)
				continue
			}

			if err := CopyFileSafely(srcPath, dstPath, dryRun); err != nil {
				return err
			}
			report.DeployedFiles = append(report.DeployedFiles, dstPath)
		}
	}
	return nil
}

// DeployPackages deploys the self-contained Gemini extension and Claude plugin
// to the stable user application directory.
// It never touches or modifies the user's config.json or custom sound directories.
func DeployPackages(repoNpmDir string, customUserAppDir string, dryRun bool) (*DeploymentReport, error) {
	userDir := customUserAppDir
	if userDir == "" {
		var err error
		userDir, err = UserAppDir()
		if err != nil {
			return nil, err
		}
	}

	geminiExtTarget := filepath.Join(userDir, "gemini-extension")
	claudePluginTarget := filepath.Join(userDir, "claude-plugin")

	report := &DeploymentReport{
		BaseDir:            userDir,
		GeminiExtensionDir: geminiExtTarget,
		ClaudePluginDir:    claudePluginTarget,
		DryRun:             dryRun,
	}

	// 1. Deploy Gemini extension files from repoNpmDir (contains gemini-extension.json, hooks/, skills/, bin/, sounds/)
	geminiManifest := filepath.Join(repoNpmDir, "gemini-extension.json")
	if _, err := os.Stat(geminiManifest); err == nil {
		geminiItems := []string{"gemini-extension.json", "bin", "hooks", "skills", "sounds", "LICENSE", "SOUND-LICENSES.md"}
		for _, item := range geminiItems {
			src := filepath.Join(repoNpmDir, item)
			dst := filepath.Join(geminiExtTarget, item)
			fi, err := os.Stat(src)
			if err != nil {
				continue
			}
			if fi.IsDir() {
				if err := CopyDirRecursive(src, dst, true, dryRun, report); err != nil {
					return nil, fmt.Errorf("deploy gemini extension dir %s: %w", item, err)
				}
			} else {
				if err := CopyFileSafely(src, dst, dryRun); err != nil {
					return nil, fmt.Errorf("deploy gemini extension file %s: %w", item, err)
				}
				report.DeployedFiles = append(report.DeployedFiles, dst)
			}
		}
	}

	// 2. Deploy Claude plugin files from repoNpmDir/claude
	claudeSrcDir := filepath.Join(repoNpmDir, "claude")
	claudeManifest := filepath.Join(claudeSrcDir, ".claude-plugin", "plugin.json")
	if _, err := os.Stat(claudeManifest); err == nil {
		if err := CopyDirRecursive(claudeSrcDir, claudePluginTarget, true, dryRun, report); err != nil {
			return nil, fmt.Errorf("deploy claude plugin: %w", err)
		}
	}

	return report, nil
}
