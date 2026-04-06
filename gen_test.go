package main_test

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDataDir = "testdata"
	genDir      = testDataDir + string(os.PathSeparator) + "gen"
	goldenDir   = testDataDir + string(os.PathSeparator) + "golden"
)

var (
	updateGolden = flag.Bool("update", false, "update golden files")
	goExec       = flag.String("go-exec", "go", "path to go executable")
)

type genTestCase struct {
	name       string
	genPath    string
	goldenPath string
}

// Run with -update flag to regenerate golden files.
func TestCodeGeneration(t *testing.T) {
	fmt.Println("running: ", os.Args[0])
	t.Parallel()

	tests := discoverGenFiles(t)
	require.NotEmpty(t, tests, "no test cases discovered")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runConversionTest(t, tt)
		})
	}
}

func discoverGenFiles(t *testing.T) []genTestCase {
	t.Helper()

	var tests []genTestCase
	err := fs.WalkDir(os.DirFS(genDir), ".",
		func(path string, d fs.DirEntry, err error) error {
			fmt.Println("discovering test case: ", path)
			if err != nil {
				return err
			}

			if d.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}

			genPath := filepath.Join(genDir, path)
			goldenPath := filepath.Join(goldenDir, path)
			testName := strings.TrimSuffix(path, ".go")
			tests = append(tests, genTestCase{
				name:       testName,
				genPath:    genPath,
				goldenPath: goldenPath,
			})

			return nil
		},
	)

	require.NoError(t, err, "failed to discover test cases")

	return tests
}

func runConversionTest(t *testing.T, tc genTestCase) {
	currentValue, err := os.ReadFile(tc.goldenPath)
	require.NoError(t, err, "failed to read current state of golden file ")

	cmd := exec.CommandContext(t.Context(), *goExec, "generate", tc.genPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err, "code generation failed for: %s", tc.genPath)

	actual, err := os.ReadFile(tc.goldenPath)
	require.NoError(t, err, "failed to read generated file: %s", tc.goldenPath)

	f, err := os.Create(tc.goldenPath)
	require.NoError(t, err, "failed to cleanup golden files")
	defer func() { _ = f.Close() }()

	_, err = f.Write(currentValue)
	require.NoError(t, err, "failed to revert golden file")

	if *updateGolden {
		t.Logf("Updated golden file: %s", tc.goldenPath)
		return
	}

	assert.Equal(t, string(currentValue), string(actual), "generated file does not match golden file: %s", tc.goldenPath)
}
