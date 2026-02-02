package hook

import (
	"fmt"
	"os"
	"path/filepath"

	// "github.com/google/uuid"
	"github.com/open-edge-platform/os-image-composer/internal/config"
	"github.com/open-edge-platform/os-image-composer/internal/utils/logger"
	"github.com/open-edge-platform/os-image-composer/internal/utils/shell"
)

func HookPostDownloadedPkgs(cachePath string, template *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Applying post-downloaded-packages hooks...")

	if template == nil {
		log.Errorf("Template cannot be nil")
		return fmt.Errorf("template cannot be nil")
	}

	// Filter to only get post-download-packages hooks
	postDownloadHooks := []config.HookScriptInfo{}
	for _, hook := range template.GetHookScriptInfo() {
		// Only include hooks that have post-download-packages fields defined and are not empty
		log.Infof("Checking hook script info: local_post_download_packages=%s, target_post_download_packages=%s", hook.LocalPostDownloadPackages, hook.TargetPostDownloadPackages)
		if hook.LocalPostDownloadPackages != "" && hook.TargetPostDownloadPackages != "" {
			postDownloadHooks = append(postDownloadHooks, hook)
			log.Infof("Found post-download-packages hook script to include: local=%s, target=%s", hook.LocalPostDownloadPackages, hook.TargetPostDownloadPackages)
		}
	}

	// Process only the post-download-packages hooks
	for _, hook := range postDownloadHooks {
		// Copy the hook script into the target rootfs
		localPath := hook.LocalPostDownloadPackages
		targetPath := fmt.Sprintf("%s/%s", cachePath, hook.TargetPostDownloadPackages)

		// Ensure the target directory exists
		targetDir := fmt.Sprintf("%s/%s", cachePath, filepath.Dir(hook.TargetPostDownloadPackages))
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			log.Errorf("Failed to create target directory for hook script: %v", err)
			return fmt.Errorf("failed to create target directory for hook script: %w", err)
		}
		log.Infof("Target directory created: %s", targetDir)

		copyCmd := fmt.Sprintf("cp %s %s", localPath, targetPath)
		if _, err := shell.ExecCmd(copyCmd, true, shell.HostPath, nil); err != nil {
			log.Errorf("Failed to copy hook script to target rootfs: %v", err)
			return fmt.Errorf("failed to copy hook script to target rootfs: %w", err)
		}
		log.Infof("Successfully copied hook script to target rootfs: %s", targetPath)

		// Now execute the hook script inside the target rootfs
		hookScriptPath := targetPath
		log.Infof("Executing post-download-packages hook script: %s", hookScriptPath)
		chmodCmd := fmt.Sprintf("chmod +x %s", hookScriptPath) //given the script executable
		if _, err := shell.ExecCmd(chmodCmd, true, shell.HostPath, nil); err != nil {
			log.Errorf("Failed to make hook script executable: %v", err)
			return fmt.Errorf("failed to make hook script executable: %w", err)
		}

		// Set up environment variable and execute the script
		env := []string{fmt.Sprintf("TARGET_CACHE=%s", cachePath)}
		hookCmd := fmt.Sprintf("sh %s", hookScriptPath)
		result, err := shell.ExecCmd(hookCmd, true, shell.HostPath, env)
		if err != nil {
			log.Errorf("Failed to execute hook script: %v", err)
			return fmt.Errorf("failed to execute hook script: %w", err)
		}
		log.Debugf("Hook script output: %s", result)
		log.Infof("Successfully executed post-download-packages hook script: %s", hookScriptPath)
	}
	return nil
}

func HookPostRootfs(installRoot string, template *config.ImageTemplate) error {
	log := logger.Logger()
	log.Infof("Applying post-rootfs hooks...")

	if template == nil {
		log.Errorf("Template cannot be nil")
		return fmt.Errorf("template cannot be nil")
	}

	// Filter to only get post-rootfs hooks (those with LocalPostRootfs and TargetPostRootfs defined)
	postRootfsHooks := []config.HookScriptInfo{}
	for _, hook := range template.GetHookScriptInfo() {
		// Only include hooks that have post-rootfs fields defined and are not empty
		if hook.LocalPostRootfs != "" && hook.TargetPostRootfs != "" {
			postRootfsHooks = append(postRootfsHooks, hook)
			log.Infof("Found post-rootfs hook script to include: local=%s, target=%s", hook.LocalPostRootfs, hook.TargetPostRootfs)
		}
	}

	// Process only the post-rootfs hooks
	for _, hook := range postRootfsHooks {
		// Copy the hook script into the target rootfs
		localPath := hook.LocalPostRootfs
		targetPath := fmt.Sprintf("%s/%s", installRoot, hook.TargetPostRootfs)

		// Ensure the target directory exists
		targetDir := fmt.Sprintf("%s/%s", installRoot, filepath.Dir(hook.TargetPostRootfs))
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			log.Errorf("Failed to create target directory for hook script: %v", err)
			return fmt.Errorf("failed to create target directory for hook script: %w", err)
		}
		log.Infof("Target directory created: %s", targetDir)

		copyCmd := fmt.Sprintf("cp %s %s", localPath, targetPath)
		if _, err := shell.ExecCmd(copyCmd, true, shell.HostPath, nil); err != nil {
			log.Errorf("Failed to copy hook script to target rootfs: %v", err)
			return fmt.Errorf("failed to copy hook script to target rootfs: %w", err)
		}
		log.Infof("Successfully copied hook script to target rootfs: %s", targetPath)

		// Now execute the hook script inside the target rootfs
		hookScriptPath := targetPath
		log.Infof("Executing post-rootfs hook script: %s", hookScriptPath)
		chmodCmd := fmt.Sprintf("chmod +x %s", hookScriptPath)
		if _, err := shell.ExecCmd(chmodCmd, true, shell.HostPath, nil); err != nil {
			log.Errorf("Failed to make hook script executable: %v", err)
			return fmt.Errorf("failed to make hook script executable: %w", err)
		}

		// Set up environment variable and execute the script
		env := []string{fmt.Sprintf("TARGET_ROOTFS=%s", installRoot)}
		hookCmd := fmt.Sprintf("sh %s", hookScriptPath)
		result, err := shell.ExecCmd(hookCmd, true, shell.HostPath, env)
		if err != nil {
			log.Errorf("Failed to execute hook script: %v", err)
			return fmt.Errorf("failed to execute hook script: %w", err)
		}
		log.Debugf("Hook script output: %s", result)
		log.Infof("Successfully executed post-rootfs hook script: %s", hookScriptPath)
	}
	return nil
}
