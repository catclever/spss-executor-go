package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunSPSS executes the given SPSS syntax in an isolated temporary directory.
func RunSPSS(spssExePath, dataFilePath, agentSyntax string, usePD bool, vmName string) (string, error) {
	// 1. Create unique temporary directory
	tempDir, err := os.MkdirTemp("", "spss_agent_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 5. Ensure cleanup runs when function exits
	defer os.RemoveAll(tempDir)

	// Translate dataFilePath if using Parallels from Mac
	effectiveDataPath := dataFilePath
	if usePD && strings.HasPrefix(dataFilePath, "/Users/") {
		effectiveDataPath = "\\\\Mac\\Host" + strings.ReplaceAll(dataFilePath, "/", "\\")
	}

	// 2. Prepare the syntax with injected GET FILE
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("GET FILE='%s'.\n", effectiveDataPath))
	sb.WriteString("EXECUTE.\n\n")
	sb.WriteString(agentSyntax)

	syntaxContent := sb.String()
	syntaxFilePath := filepath.Join(tempDir, "script.sps")

	if err := os.WriteFile(syntaxFilePath, []byte(syntaxContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write syntax file: %w", err)
	}

	var cmd *exec.Cmd

	if usePD {
		// Parallels Desktop VM execution path translation
		// E.g., /tmp/spss_agent_xxxx/script.sps -> \\Mac\Host\private\tmp\spss_agent_xxxx\script.sps
		// Note: Mac's /tmp is actually a symlink to /private/tmp. MkdirTemp usually returns /var/... or /private/var...
		pdSyntaxPath := "\\\\Mac\\Host" + strings.ReplaceAll(syntaxFilePath, "/", "\\")

		// prlctl exec "Windows 11" "C:\Program Files\IBM\SPSS Statistics\28\stats.exe" -batch -f "\\Mac\Host\private\var\..."
		cmd = exec.Command("prlctl", "exec", vmName, spssExePath, "-batch", "-f", pdSyntaxPath)
	} else {
		// Native execution (Windows or Mac natively)
		cmd = exec.Command(spssExePath, "-batch", "-f", syntaxFilePath)
		cmd.Dir = tempDir
	}

	// Capture output
	out, err := cmd.CombinedOutput()
	outputStr := string(out)

	// 4. Handle execution outcome
	if err != nil {
		return outputStr, fmt.Errorf("SPSS execution failed: %v", err)
	}

	return outputStr, nil
}
