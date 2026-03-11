package xmlparser

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// InjectBackendListener reads a JMX file and appends an InfluxDB BackendListener
// inside the main ThreadGroup or just before the closing hashTree, if one does not already exist.
func InjectBackendListener(jmxPath string) error {
	content, err := os.ReadFile(jmxPath)
	if err != nil {
		return fmt.Errorf("failed to read JMX file: %w", err)
	}

	xmlStr := string(content)

	// If it already has an InfluxDB BackendListener, do not inject a redundant one.
	// This allows other custom BackendListeners (like Kafka) to coexist.
	if strings.Contains(xmlStr, "org.apache.jmeter.visualizers.backend.influxdb.InfluxdbBackendListenerClient") {
		return nil
	}

	// The listener XML to inject (uses variables configured by our executor args)
	listenerXML := `
      <BackendListener guiclass="BackendListenerGui" testclass="BackendListener" testname="Backend Listener (Auto-Injected)" enabled="true">
        <elementProp name="arguments" elementType="Arguments" guiclass="ArgumentsPanel" testclass="Arguments" enabled="true">
          <collectionProp name="Arguments.arguments">
            <elementProp name="influxdbMetricsSender" elementType="Argument">
              <stringProp name="Argument.name">influxdbMetricsSender</stringProp>
              <stringProp name="Argument.value">${__P(influxdbMetricsSender)}</stringProp>
              <stringProp name="Argument.metadata">=</stringProp>
            </elementProp>
            <elementProp name="influxdbUrl" elementType="Argument">
              <stringProp name="Argument.name">influxdbUrl</stringProp>
              <stringProp name="Argument.value">${__P(influxdbUrl)}</stringProp>
              <stringProp name="Argument.metadata">=</stringProp>
            </elementProp>
            <elementProp name="application" elementType="Argument">
              <stringProp name="Argument.name">application</stringProp>
              <stringProp name="Argument.value">${__P(application)}</stringProp>
              <stringProp name="Argument.metadata">=</stringProp>
            </elementProp>
            <elementProp name="measurement" elementType="Argument">
              <stringProp name="Argument.name">measurement</stringProp>
              <stringProp name="Argument.value">jmeter</stringProp>
              <stringProp name="Argument.metadata">=</stringProp>
            </elementProp>
            <elementProp name="summaryOnly" elementType="Argument">
              <stringProp name="Argument.name">summaryOnly</stringProp>
              <stringProp name="Argument.value">${__P(summaryOnly)}</stringProp>
              <stringProp name="Argument.metadata">=</stringProp>
            </elementProp>
            <elementProp name="samplersRegex" elementType="Argument">
              <stringProp name="Argument.name">samplersRegex</stringProp>
              <stringProp name="Argument.value">.*</stringProp>
              <stringProp name="Argument.metadata">=</stringProp>
            </elementProp>
            <elementProp name="percentiles" elementType="Argument">
              <stringProp name="Argument.name">percentiles</stringProp>
              <stringProp name="Argument.value">90;95;99</stringProp>
              <stringProp name="Argument.metadata">=</stringProp>
            </elementProp>
            <elementProp name="testTitle" elementType="Argument">
              <stringProp name="Argument.name">testTitle</stringProp>
              <stringProp name="Argument.value">Test name</stringProp>
              <stringProp name="Argument.metadata">=</stringProp>
            </elementProp>
            <elementProp name="eventTags" elementType="Argument">
              <stringProp name="Argument.name">eventTags</stringProp>
              <stringProp name="Argument.value"></stringProp>
              <stringProp name="Argument.metadata">=</stringProp>
            </elementProp>
          </collectionProp>
        </elementProp>
        <stringProp name="classname">org.apache.jmeter.visualizers.backend.influxdb.InfluxdbBackendListenerClient</stringProp>
      </BackendListener>
      <hashTree/>`

	// Robust injection: strip the outermost two </hashTree> and </jmeterTestPlan> closing tags
	// from the end of the file, append the BackendListener inside the main test plan tree,
	// then re-close the document. This handles the standard JMX structure safely.
	cleanXML := strings.TrimSpace(xmlStr)
	if strings.HasSuffix(cleanXML, "</jmeterTestPlan>") {
		// remove </jmeterTestPlan>
		cleanXML = cleanXML[:len(cleanXML)-len("</jmeterTestPlan>")]
		cleanXML = strings.TrimSpace(cleanXML)

		if strings.HasSuffix(cleanXML, "</hashTree>") {
			cleanXML = cleanXML[:len(cleanXML)-len("</hashTree>")]
			cleanXML = strings.TrimSpace(cleanXML)

			if strings.HasSuffix(cleanXML, "</hashTree>") {
				cleanXML = cleanXML[:len(cleanXML)-len("</hashTree>")]
				// Now we are inside the main test plan tree.

				var finalBuf bytes.Buffer
				finalBuf.WriteString(cleanXML)
				finalBuf.WriteString("\n")
				finalBuf.WriteString(listenerXML)
				finalBuf.WriteString("\n  </hashTree>\n  </hashTree>\n</jmeterTestPlan>\n")

				return os.WriteFile(jmxPath, finalBuf.Bytes(), 0644)
			}
		}
	}

	return fmt.Errorf("failed to inject backend listener: could not find proper injection point at EOF")
}
