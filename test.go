func ReplacePlaceholdersInFile(placeholder, value, filePath string) error {
	sedCmd := fmt.Sprintf("sed -i 's|%s|%s|g' %s", placeholder, value, filePath)
	if _, err := shell.ExecCmd(sedCmd, true, "", nil); err != nil {
		return fmt.Errorf("failed to replace placeholder %s with %s in file %s: %w", placeholder, value, filePath, err)
	}
	return nil
}
