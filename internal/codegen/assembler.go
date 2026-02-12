package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CompileToExecutable takes assembly code and produces a runnable binary
func CompileToExecutable(assembly string, outputPath string) error {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "twice-compile-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write assembly to file
	asmPath := filepath.Join(tmpDir, "program.s")
	if err := os.WriteFile(asmPath, []byte(assembly), 0o644); err != nil {
		return fmt.Errorf("failed to write assembly: %v", err)
	}

	// Assemble: as program.s -o program.o
	objPath := filepath.Join(tmpDir, "program.o")
	cmd := exec.Command("as", asmPath, "-o", objPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("assembler failed: %v\n%s", err, output)
	}

	// Link: ld program.o -o output (static) or use gcc
	// Using gcc is easier as it handles C runtime linking
	cmd = exec.Command("gcc", "-static", "-nostartfiles", objPath, "-o", outputPath)
	if _, err := cmd.CombinedOutput(); err != nil {
		// Fallback to ld for pure assembly
		cmd = exec.Command("ld", objPath, "-o", outputPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("linker failed: %v\n%s", err, output)
		}
	}

	return nil
}
