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
	// Create inside the current working directory to guarantee Parallels VM can access it via the \Mac\Host share (system /tmp is often not shared)
	cwd, _ := os.Getwd()
	tempDir, err := os.MkdirTemp(cwd, "spss_agent_*")
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

	// Translate output text file path if using Parallels from Mac
	outputTxtPath := filepath.Join(tempDir, "output.txt")
	effectiveOutputPath := outputTxtPath
	if usePD {
		effectiveOutputPath = "\\\\Mac\\Host" + strings.ReplaceAll(outputTxtPath, "/", "\\")
	}

	// 2. Prepare the syntax with injected GET FILE and OMS
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("GET FILE='%s'.\n", effectiveDataPath))
	sb.WriteString(fmt.Sprintf("OMS /SELECT ALL /DESTINATION FORMAT=TEXT OUTFILE='%s'.\n", effectiveOutputPath))
	sb.WriteString("EXECUTE.\n\n")
	sb.WriteString(agentSyntax)
	sb.WriteString("\nOMSEND.\n")

	syntaxContent := sb.String()
	syntaxFilePath := filepath.Join(tempDir, "script.sps")

	if err := os.WriteFile(syntaxFilePath, []byte(syntaxContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write syntax file: %w", err)
	}

	// 3. Determine Execution Path (statisticsb vs stats -production silent)
	var cmd *exec.Cmd

	isWindows := strings.HasSuffix(strings.ToLower(spssExePath), ".exe")
	batchExeName := "statisticsb"
	if isWindows {
		batchExeName = "statisticsb.exe"
	}

	batchExePath := filepath.Join(filepath.Dir(spssExePath), batchExeName)
	hasBatch := false

	if usePD {
		checkCmdStr := fmt.Sprintf("if exist \"%s\" (exit 0) else (exit 1)", batchExePath)
		if err := exec.Command("prlctl", "exec", vmName, "cmd.exe", "/c", checkCmdStr).Run(); err == nil {
			hasBatch = true
		}
	} else {
		if _, err := os.Stat(batchExePath); err == nil {
			hasBatch = true
		}
	}

	if hasBatch {
		// Use true Batch Facility (statisticsb)
		if usePD {
			pdSyntaxPath := "\\\\Mac\\Host" + strings.ReplaceAll(syntaxFilePath, "/", "\\")
			cmd = exec.Command("prlctl", "exec", vmName, batchExePath, "-f", pdSyntaxPath)
		} else {
			cmd = exec.Command(batchExePath, "-f", syntaxFilePath)
			cmd.Dir = tempDir
		}
	} else {
		// Use Fallback Production Job XML for stats.exe
		spjFilePath := filepath.Join(tempDir, "job.spj")
		effectiveSpjPath := spjFilePath
		targetSyntaxForSpj := syntaxFilePath

		if usePD {
			effectiveSpjPath = "\\\\Mac\\Host" + strings.ReplaceAll(spjFilePath, "/", "\\")
			targetSyntaxForSpj = "\\\\Mac\\Host" + strings.ReplaceAll(syntaxFilePath, "/", "\\")
		}

		spjContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<job error-stop="false" syntax-format="interactive" syntax-symbol="PRAGMA">
  <output clear="false" print-output="false" print-syntax="false" print-error="false" show-charts="false" export-output="false" />
  <syntax-list>
    <syntax file="%s" />
  </syntax-list>
</job>`, targetSyntaxForSpj)

		if err := os.WriteFile(spjFilePath, []byte(spjContent), 0644); err != nil {
			return "", fmt.Errorf("failed to write spj file: %w", err)
		}

		if usePD {
			cmd = exec.Command("prlctl", "exec", vmName, spssExePath, "-production", "silent", effectiveSpjPath)
		} else {
			cmd = exec.Command(spssExePath, "-production", "silent", spjFilePath)
			cmd.Dir = tempDir
		}
	}

	// Capture command standard output as a fallback
	out, err := cmd.CombinedOutput()
	outputStr := string(out)

	// 4. Read the OMS generated text file
	var resultOutput string
	outputBytes, readErr := os.ReadFile(outputTxtPath)
	if readErr == nil {
		resultOutput = string(outputBytes)
	} else {
		resultOutput = fmt.Sprintf("Failed to read SPSS output file: %v\n---\nRaw Execution Output:\n%s", readErr, outputStr)
	}

	if err != nil {
		return resultOutput, fmt.Errorf("SPSS execution failed: %v\n[Stdout]: %s", err, outputStr)
	}

	return resultOutput, nil
}
