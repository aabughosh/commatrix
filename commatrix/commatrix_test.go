package commatrix

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/openshift-kni/commatrix/types"
	"github.com/stretchr/testify/assert"
)

func TestGetPrintFunction(t *testing.T) {
	tests := []struct {
		format         string
		expectedFnType string
		expectedErr    bool
	}{
		{"json", "func(types.ComMatrix) ([]byte, error)", false},
		{"csv", "func(types.ComMatrix) ([]byte, error)", false},
		{"yaml", "func(types.ComMatrix) ([]byte, error)", false},
		{"nft", "func(types.ComMatrix) ([]byte, error)", false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			fn, err := getPrintFunction(tt.format)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, fmt.Sprintf("%T", fn), tt.expectedFnType)
			}
		})
	}
}
func TestCreateOutputFiles(t *testing.T) {
	destDir := t.TempDir()

	tcpFile, udpFile, err := createOutputFiles(destDir)
	assert.NoError(t, err)
	defer tcpFile.Close()
	defer udpFile.Close()

	assert.FileExists(t, filepath.Join(destDir, "raw-ss-tcp"))
	assert.FileExists(t, filepath.Join(destDir, "raw-ss-udp"))
}

func TestWriteMatrixToFile(t *testing.T) {
	destDir := t.TempDir()
	matrix := types.ComMatrix{
		Matrix: []types.ComDetails{
			{NodeRole: "master", Service: "testService"},
		},
	}
	printFn := types.ToJSON
	fileName := "test-matrix"
	format := "json"

	err := writeMatrixToFile(matrix, fileName, format, printFn, destDir)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(destDir, "test-matrix.json"))
}
