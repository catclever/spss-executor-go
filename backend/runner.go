package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunSPSS executes the given SPSS syntax in an isolated temporary directory.
func RunSPSS(ctx context.Context, spssExePath, dataFilePath, agentSyntax string, usePD bool, vmName string) (string, error) {
	// 1. Create unique temporary directory
	// Use standard temp directory but ensure it doesn't conflict with Wails watcher,
	// and is fully accessible by standard macOS permissions (not hidden).
	baseTempDir := filepath.Join(os.TempDir(), "spss_workspace")
	os.MkdirAll(baseTempDir, 0755)

	tempDir, err := os.MkdirTemp(baseTempDir, "run_*")
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

	// 2. Prepare the syntax with injected GET FILE, PRINTBACK prevention, and OMS
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("GET FILE='%s'.\n", effectiveDataPath))
	sb.WriteString("SET PRINTBACK=YES.\n")
	sb.WriteString(fmt.Sprintf("OMS /SELECT ALL /DESTINATION FORMAT=TEXT OUTFILE='%s'.\n", effectiveOutputPath))
	sb.WriteString("EXECUTE.\n\n")
	sb.WriteString(agentSyntax)
	sb.WriteString("\nECHO \"AGENT_EXECUTION_COMPLETE\".\n")
	sb.WriteString("OMSEND.\n")
	sb.WriteString("FINISH.\n")

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
		if err := exec.CommandContext(ctx, "prlctl", "exec", vmName, "cmd.exe", "/c", checkCmdStr).Run(); err == nil {
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
			cmd = exec.CommandContext(ctx, "prlctl", "exec", vmName, batchExePath, "-f", pdSyntaxPath)
		} else {
			cmd = exec.CommandContext(ctx, batchExePath, "-f", syntaxFilePath)
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

		dummySpvPath := filepath.Join(tempDir, "dummy.spv")
		if usePD {
			dummySpvPath = "\\\\Mac\\Host" + strings.ReplaceAll(dummySpvPath, "/", "\\")
		}

		spjContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<job xmlns="http://www.ibm.com/software/analytics/spss/xml/production" syntaxErrorHandling="continue" syntaxFormat="interactive" unicode="true">
  <output outputFormat="viewer" outputPath="%s"/>
  <syntax syntaxPath="%s"/>
</job>`, dummySpvPath, targetSyntaxForSpj)

		if err := os.WriteFile(spjFilePath, []byte(spjContent), 0644); err != nil {
			return "", fmt.Errorf("failed to write spj file: %w", err)
		}

		if usePD {
			cmd = exec.CommandContext(ctx, "prlctl", "exec", vmName, spssExePath, "-production", "silent", effectiveSpjPath)
		} else {
			// On Mac, launching the binary directly within a Go shell pipe often crashes the WindowServer connection (Mach port error).
			// We use the native macOS `open` command with `-W` (Wait) and `-n` (New Instance) to provide a proper UI context to the silent run.
			if appIndex := strings.Index(strings.ToLower(spssExePath), ".app"); appIndex != -1 {
				appBundlePath := spssExePath[:appIndex+4]
				cmd = exec.CommandContext(ctx, "open", "-W", "-n", "-a", appBundlePath, "--args", "-production", "silent", spjFilePath)
			} else {
				cmd = exec.CommandContext(ctx, spssExePath, "-production", "silent", spjFilePath)
			}
			cmd.Dir = tempDir
		}
	}

	if hasBatch {
		// Capture command standard output for batch execution
		out, err := cmd.CombinedOutput()
		outputStr := string(out)

		var resultOutput string
		outputBytes, readErr := os.ReadFile(outputTxtPath)
		if readErr == nil {
			resultOutput = string(outputBytes)
		} else {
			resultOutput = fmt.Sprintf("Failed to read SPSS output file: %v\n---\nRaw Execution Output:\n%s", readErr, outputStr)
		}

		if err != nil {
			return resultOutput, fmt.Errorf("SPSS batch execution failed: %v\n[Stdout]: %s", err, outputStr)
		}
		return resultOutput, nil
	}

	// For stats.exe fallback:
	// On Windows, stats.exe returns immediately while launching a background process.
	// On Mac, 'open -W' hangs indefinitely even after SPSS is done.
	// So we must actively poll output.txt for the completion marker.
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start SPSS fallback command: %w", err)
	}

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if b, err := os.ReadFile(outputTxtPath); err == nil {
					if strings.Contains(string(b), "AGENT_EXECUTION_COMPLETE") {
						close(done)
						return
					}
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	select {
	case <-done:
		// File written successfully
	case <-ctx.Done():
		return "", fmt.Errorf("execution cancelled by context")
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("SPSS execution timed out after 5 minutes waiting for output.txt")
	}

	// Cleanup process
	if !isWindows && !usePD {
		exec.Command("pkill", "-9", "-f", "SPSS Statistics").Run()
	} else if isWindows && !usePD {
		exec.Command("taskkill", "/F", "/IM", "stats.exe").Run()
	} else if usePD {
		exec.Command("prlctl", "exec", vmName, "cmd.exe", "/c", "taskkill /F /IM stats.exe").Run()
	}

	go cmd.Wait() // reap zombie process without blocking

	// Read the final file
	outputBytes, readErr := os.ReadFile(outputTxtPath)
	if readErr != nil {
		return "", fmt.Errorf("failed to read SPSS output file after completion: %v", readErr)
	}
	return string(outputBytes), nil
}
