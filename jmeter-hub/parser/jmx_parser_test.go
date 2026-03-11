package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseRequiredCSVs(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		jmxContent  string
		expected    []string
		expectError bool
	}{
		{
			name: "Single CSVDataSet",
			jmxContent: `
				<jmeterTestPlan version="1.2" properties="5.0" jmeter="5.6.2">
				  <hashTree>
					<CSVDataSet guiclass="TestBeanGUI" testclass="CSVDataSet" testname="CSV Data Set Config" enabled="true">
					  <stringProp name="filename">${__P(users_csv)}</stringProp>
					</CSVDataSet>
				  </hashTree>
				</jmeterTestPlan>
			`,
			expected: []string{"users_csv"},
		},
		{
			name: "With Default Property",
			jmxContent: `
				<jmeterTestPlan version="1.2" properties="5.0" jmeter="5.6.2">
				  <hashTree>
					<CSVDataSet guiclass="TestBeanGUI" testclass="CSVDataSet" testname="CSV Data Set Config" enabled="true">
					  <stringProp name="filename">${__P(data_csv, default.csv)}</stringProp>
					</CSVDataSet>
				  </hashTree>
				</jmeterTestPlan>
			`,
			expected: []string{"data_csv"},
		},
		{
			name: "Multiple Deduplicated",
			jmxContent: `
				<jmeterTestPlan version="1.2" properties="5.0" jmeter="5.6.2">
				  <hashTree>
					<CSVDataSet guiclass="TestBeanGUI" testclass="CSVDataSet" testname="CSV Data Set Config" enabled="true">
					  <stringProp name="filename">${__P(data_csv)}</stringProp>
					</CSVDataSet>
					<CSVDataSet guiclass="TestBeanGUI" testclass="CSVDataSet" testname="CSV Data Set Config 2" enabled="true">
					  <stringProp name="filename">${__P(data_csv)}</stringProp>
					</CSVDataSet>
					<CSVDataSet guiclass="TestBeanGUI" testclass="CSVDataSet" testname="CSV Data Set Config 3" enabled="true">
					  <stringProp name="filename">${__P(users_csv, default.csv)}</stringProp>
					</CSVDataSet>
				  </hashTree>
				</jmeterTestPlan>
			`,
			expected: []string{"data_csv", "users_csv"},
		},
		{
			name: "Outside CSVDataSet should be ignored",
			jmxContent: `
				<jmeterTestPlan version="1.2" properties="5.0" jmeter="5.6.2">
				  <hashTree>
					<CSVDataSet guiclass="TestBeanGUI" testclass="CSVDataSet" testname="CSV Data Set Config" enabled="true">
					  <stringProp name="filename">${__P(data_csv)}</stringProp>
					</CSVDataSet>
					<ThreadGroup guiclass="ThreadGroupGui" testclass="ThreadGroup" testname="Thread Group" enabled="true">
					  <stringProp name="filename">${__P(ignored_csv)}</stringProp>
					</ThreadGroup>
				  </hashTree>
				</jmeterTestPlan>
			`,
			expected: []string{"data_csv"},
		},
		{
			name: "No CSV required",
			jmxContent: `
				<jmeterTestPlan version="1.2" properties="5.0" jmeter="5.6.2">
				  <hashTree>
					<ThreadGroup guiclass="ThreadGroupGui" testclass="ThreadGroup" testname="Thread Group" enabled="true">
					</ThreadGroup>
				  </hashTree>
				</jmeterTestPlan>
			`,
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jmxPath := filepath.Join(tempDir, tc.name+".jmx")
			err := os.WriteFile(jmxPath, []byte(tc.jmxContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			result, err := ParseRequiredCSVs(jmxPath)
			if (err != nil) != tc.expectError {
				t.Errorf("Expected error: %v, got: %v", tc.expectError, err)
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected: %v, got: %v", tc.expected, result)
			}
		})
	}
}
