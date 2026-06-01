package cmd

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

type junitTestSuites struct {
	XMLName   xml.Name         `xml:"testsuites"`
	Tests     int              `xml:"tests,attr"`
	Failures  int              `xml:"failures,attr"`
	TestSuite []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Name      string        `xml:"name,attr"`
	Tests     int           `xml:"tests,attr"`
	Failures  int           `xml:"failures,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string         `xml:"name,attr"`
	Classname string         `xml:"classname,attr"`
	Failure   *junitFailure  `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

func printDatasetGateJUnit(result map[string]any, exitCode int) error {
	gate := mapObject(result, "gate")
	if gate == nil {
		return fmt.Errorf("gate response missing gate payload")
	}

	regressions := mapSlice(gate, "regressions")
	failedThresholds := mapSlice(gate, "failed_thresholds")
	failures := len(regressions) + len(failedThresholds)
	tests := len(regressions)
	if tests == 0 && len(failedThresholds) > 0 {
		tests = len(failedThresholds)
	}
	if tests == 0 {
		tests = 1
	}

	cases := make([]junitTestCase, 0, tests)
	for _, item := range regressions {
		row, _ := item.(map[string]any)
		if row == nil {
			continue
		}
		exampleID := mapString(row, "dataset_example_id")
		reason := mapString(row, "reason")
		message := fmt.Sprintf("%s baseline=%s candidate=%s", reason, mapString(row, "baseline_verdict"), mapString(row, "candidate_verdict"))
		cases = append(cases, junitTestCase{
			Name:      exampleID,
			Classname: "dataset-gate." + reason,
			Failure:   &junitFailure{Message: reason, Body: message},
		})
	}
	for _, item := range failedThresholds {
		threshold, _ := item.(string)
		if strings.TrimSpace(threshold) == "" {
			continue
		}
		cases = append(cases, junitTestCase{
			Name:      threshold,
			Classname: "dataset-gate.threshold",
			Failure:   &junitFailure{Message: threshold, Body: fmt.Sprintf("threshold %s failed (pass_rate=%s regressions=%s)", threshold, mapString(gate, "pass_rate"), mapString(gate, "regression_count"))},
		})
	}
	if len(cases) == 0 {
		cases = append(cases, junitTestCase{
			Name:      "dataset-gate",
			Classname: "dataset-gate",
		})
	}

	payload := junitTestSuites{
		Tests:    tests,
		Failures: failures,
		TestSuite: []junitTestSuite{{
			Name:      "dataset-gate",
			Tests:     tests,
			Failures:  failures,
			TestCases: cases,
		}},
	}
	encoded, err := xml.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", append([]byte(xml.Header), encoded...))
	if exitCode != 0 {
		datasetGateExit(exitCode)
	}
	return nil
}

var datasetGateExit = os.Exit
