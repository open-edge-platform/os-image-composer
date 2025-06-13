package chroot

import "fmt"

func InitChrootEnv(targetOS, targetDist, targetArch string) error {
	err := BuildChrootEnv(targetOS, targetDist, targetArch)
	if err != nil {
		return fmt.Errorf("failed to build chroot environment: %w", err)
	}
	return nil
}

func CleanupChrootEnv(targetOS, targetDist, targetArch string) error {
	return nil
}
