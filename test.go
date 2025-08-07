// Append appends a string to the end of file dst.
func Append(data string, dst string) error {
	dstFile, err := os.OpenFile(dst, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s for appending: %w", dst, err)
	}
	defer dstFile.Close()

	_, err = dstFile.WriteString(data)
	return err
}
