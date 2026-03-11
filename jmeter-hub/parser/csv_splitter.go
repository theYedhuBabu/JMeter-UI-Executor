package parser

import (
	"encoding/csv"
	"fmt"
	"os"
)

// SplitCSV reads a CSV file and divides its rows evenly into numChunks parts.
// It returns a slice of 2D string arrays (the chunks). Each chunk is a [][]string.
// If hasHeader is true, the first row of the CSV is prepended to each chunk.
func SplitCSV(filePath string, numChunks int, hasHeader bool) ([][][]string, error) {
	if numChunks <= 0 {
		return nil, fmt.Errorf("numChunks must be greater than 0")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Read all rows
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	totalRows := len(rows)

	// If the file is empty, return empty chunks
	if totalRows == 0 {
		emptyChunks := make([][][]string, numChunks)
		for i := 0; i < numChunks; i++ {
			emptyChunks[i] = [][]string{}
		}
		return emptyChunks, nil
	}

	var headerRow []string
	dataRows := rows

	if hasHeader && totalRows > 0 {
		headerRow = rows[0]
		dataRows = rows[1:]
		totalRows = len(dataRows)
	}

	// Base sizes are calculated strictly on dataRows
	baseSize := totalRows / numChunks
	remainder := totalRows % numChunks

	chunks := make([][][]string, numChunks)
	currentRow := 0

	for i := 0; i < numChunks; i++ {
		// Distribute remainder one by one to the earlier chunks
		size := baseSize
		if remainder > 0 {
			size++
			remainder--
		}

		endRow := currentRow + size
		if endRow > totalRows {
			endRow = totalRows
		}

		// Ensure we create a new slice even if it is empty to avoid nil
		var chunk [][]string
		if hasHeader && headerRow != nil {
			chunk = append(chunk, headerRow)
		}

		if size > 0 {
			chunk = append(chunk, dataRows[currentRow:endRow]...)
		}

		chunks[i] = chunk
		currentRow = endRow
	}

	return chunks, nil
}

// SaveCSVChunk writes a 2D string array chunk back to a CSV file on disk.
func SaveCSVChunk(chunk [][]string, destPath string) error {
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	for _, row := range chunk {
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	// Flush explicitly so we can check the error (defer would swallow it)
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("csv flush error: %w", err)
	}

	return nil
}
