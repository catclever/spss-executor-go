package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunSPSS executes the given SPSS syntax in an isolated temporary directory.
func RunSPSS(spssExePath, dataFilePath, agentSyntax string) (string, error) {
	// 1. Create unique temporary directory
	tempDir, err := os.MkdirTemp("", "spss_agent_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 5. Ensure cleanup runs when function exits
	defer os.RemoveAll(tempDir)

	// 2. Prepare the syntax with injected GET FILE
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("GET FILE='%s'.\n", dataFilePath))
	sb.WriteString("EXECUTE.\n\n")
	sb.WriteString(agentSyntax)

	syntaxContent := sb.String()
	syntaxFilePath := filepath.Join(tempDir, "script.sps")

	if err := os.WriteFile(syntaxFilePath, []byte(syntaxContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write syntax file: %w", err)
	}

	// 3. Execute SPSS in batch mode
	// Typical SPSS command: stats -batch -f script.sps
	cmd := exec.Command(spssExePath, "-batch", "-f", syntaxFilePath)
	
	// Set the working directory to the temporary directory
	cmd.Dir = tempDir

	// Capture output
	out, err := cmd.CombinedOutput()
	outputStr := string(out)

	// 4. Handle execution outcome
	if err != nil {
		return outputStr, fmt.Errorf("SPSS execution failed: %v", err)
	}

	return outputStr, nil
}
