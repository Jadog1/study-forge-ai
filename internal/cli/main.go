package cli

// Execute runs the shared CLI command tree.
func Execute() error {
	return rootCmd.Execute()
}
