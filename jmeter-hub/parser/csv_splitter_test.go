package parser

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	tempDir := t.TempDir()
	csvPath := filepath.Join(tempDir, "test.csv")

	// Create test CSV
	rows := [][]string{
		{"id", "name"},
		{"1", "Alice"},
		{"2", "Bob"},
		{"3", "Charlie"},
		{"4", "David"},
		{"5", "Eve"},
	}

	SaveCSVChunk(rows, csvPath) // using our own helper

	tests := []struct {
		name      string
		numChunks int
		hasHeader bool
		expectErr bool
		expected  [][][]string
	}{
		{
			name:      "Split into 2 chunks evenly with header",
			numChunks: 2,
			hasHeader: true,
			expected: [][][]string{
				{{"id", "name"}, {"1", "Alice"}, {"2", "Bob"}, {"3", "Charlie"}},
				{{"id", "name"}, {"4", "David"}, {"5", "Eve"}},
			},
		},
		{
			name:      "Split into 2 chunks evenly without header",
			numChunks: 2,
			hasHeader: false,
			expected: [][][]string{
				{{"id", "name"}, {"1", "Alice"}, {"2", "Bob"}},
				{{"3", "Charlie"}, {"4", "David"}, {"5", "Eve"}},
			},
		},
		{
			name:      "Split into 3 chunks with header",
			numChunks: 3,
			hasHeader: true,
			expected: [][][]string{
				{{"id", "name"}, {"1", "Alice"}, {"2", "Bob"}},
				{{"id", "name"}, {"3", "Charlie"}, {"4", "David"}},
				{{"id", "name"}, {"5", "Eve"}},
			},
		},
		{
			name:      "Split into 4 chunks with header",
			numChunks: 4,
			hasHeader: true,
			expected: [][][]string{
				{{"id", "name"}, {"1", "Alice"}, {"2", "Bob"}},
				{{"id", "name"}, {"3", "Charlie"}},
				{{"id", "name"}, {"4", "David"}},
				{{"id", "name"}, {"5", "Eve"}},
			},
		},
		{
			name:      "Split into more chunks than rows with header",
			numChunks: 8,
			hasHeader: true,
			expected: [][][]string{
				{{"id", "name"}, {"1", "Alice"}},
				{{"id", "name"}, {"2", "Bob"}},
				{{"id", "name"}, {"3", "Charlie"}},
				{{"id", "name"}, {"4", "David"}},
				{{"id", "name"}, {"5", "Eve"}},
				{{"id", "name"}},
				{{"id", "name"}},
				{{"id", "name"}},
			},
		},
		{
			name:      "Invalid num chunks",
			numChunks: 0,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chunks, err := SplitCSV(csvPath, tc.numChunks, tc.hasHeader)
			if (err != nil) != tc.expectErr {
				t.Fatalf("Expected error: %v, got: %v", tc.expectErr, err)
			}

			if !tc.expectErr {
				if len(chunks) != tc.numChunks {
					t.Errorf("Expected %d chunks, got %d", tc.numChunks, len(chunks))
				}
				if !reflect.DeepEqual(chunks, tc.expected) {
					t.Errorf("Expected chunks: %v, got: %v", tc.expected, chunks)
				}
			}
		})
	}
}

func TestSaveCSVChunk(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "out.csv")

	chunk := [][]string{
		{"col1", "col2"},
		{"v1", "v2"},
	}

	err := SaveCSVChunk(chunk, outPath)
	if err != nil {
		t.Fatalf("Failed to write chunk: %v", err)
	}

	// Verify
	file, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("Failed to open written file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	readChunk, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read back chunk: %v", err)
	}

	if !reflect.DeepEqual(chunk, readChunk) {
		t.Errorf("Written data %v does not match read data %v", chunk, readChunk)
	}
}
