// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestOptions() Options {
	options := newOptions()
	options.rawArgs = []string{}
	options.captureOutput = func(string, ...string) ([]string, error) {
		panic("captureOutput was called without being set by the test")
	}
	return options
}

func TestMakeExampleConfig(t *testing.T) {
	expected := strings.Split(strings.TrimLeft(strings.ReplaceAll(`
comment: Comment is not interpreted or used; it is provided as a structured way of
	adding comments to a config, so that automated editing is easier.
default_coverage: 80
rules:
- comment: Low coverage is acceptable for main()
	filename_regex: ""
	function_regex: ^main$
	receiver_regex: ""
	coverage: 50
- comment: All the fooOrDie() functions should be fully tested because they panic()
		on failure
	filename_regex: ""
	function_regex: OrDie$
	receiver_regex: ""
	coverage: 100
- comment: 'TODO: improve test coverage for parse_json.go'
	filename_regex: ^parse_json.go$
	function_regex: ""
	receiver_regex: ""
	coverage: 73
- comment: Full coverage for other parsers
	filename_regex: ^parse.*.go$
	function_regex: ""
	receiver_regex: ""
	coverage: 100
- comment: Url.String() has low coverage
	filename_regex: ^urls.go$
	function_regex: ^String$
	receiver_regex: ^Url$
	coverage: 56
- comment: String() everywhere else should have high coverage
	filename_regex: ""
	function_regex: ^String$
	receiver_regex: ""
	coverage: 100
`, "\t", "  "), "\n"), "\n")
	actual := strings.Split(makeExampleConfig(), "\n")
	assert.Equal(t, expected, actual)
}

func TestGenerateConfig(t *testing.T) {
	coverage := []CoverageLine{
		{
			Filename:   "test.go",
			LineNumber: "1",
			Function:   "func1",
			Coverage:   20.0,
		},
		{
			Filename:   "test.go",
			LineNumber: "2",
			Function:   "func5",
			Coverage:   34.0,
		},
		{
			Filename:   "test.go",
			LineNumber: "9",
			Function:   "func17",
			Coverage:   12.3,
		},
	}

	expected := Config{
		DefaultCoverage: 100.0,
		Rules: []Rule{
			{
				FilenameRegex: "^test.go$",
				FunctionRegex: "^func1$",
				Comment:       "Generated rule for func1, found at test.go:1",
				Coverage:      20.0,
			},
			{
				FilenameRegex: "^test.go$",
				FunctionRegex: "^func5$",
				Comment:       "Generated rule for func5, found at test.go:2",
				Coverage:      34.0,
			},
			{
				FilenameRegex: "^test.go$",
				FunctionRegex: "^func17$",
				Comment:       "Generated rule for func17, found at test.go:9",
				Coverage:      12.3,
			},
		},
	}

	generated := generateConfig(coverage)
	assert.Equal(t, expected, generated)
}

func TestParseYAMLConfigErrors(t *testing.T) {
	table := []struct {
		input string
		err   string
	}{
		{
			err:   "failed parsing YAML",
			input: "asdf",
		},
		{
			err:   "default coverage (101.0) is outside the range 0-100",
			input: "default_coverage: 101",
		},
		{
			err: "coverage (1234.0) is outside the range 0-100 in",
			input: `
							default_coverage: 99
							rules:
							- filename_regex: asdf
							!!coverage: 1234
						`,
		},
		{
			err: "coverage (-1.0) is outside the range 0-100 in",
			input: `
							default_coverage: 99
							rules:
							- filename_regex: asdf
							!!coverage: -1`,
		},
		{
			err: "every regex is an empty string in rule",
			input: `
							default_coverage: 99
							rules:
							- coverage: 1`,
		},
	}
	for _, test := range table {
		// This is ugly but it's the only way I've found to get reasonable
		// indentation.
		yml := strings.ReplaceAll(test.input, "\t", "")
		yml = strings.ReplaceAll(yml, "!!", "  ")
		_, err := parseYAMLConfig([]byte(yml))
		if assert.Error(t, err, test.input) {
			// Note: the error message seems mangled when it's printed here, but it's
			// fine when printed for real.  I don't understand why and an hour of
			// debugging has gotten me nowhere :(
			assert.Contains(t, err.Error(), test.err, yml)
		}
	}
}

func TestParseYAMLConfigSuccess(t *testing.T) {
	config, err := parseYAMLConfig([]byte(""))
	assert.Nil(t, err)
	assert.Equal(t, 0.0, config.DefaultCoverage)

	yml := `
default_coverage: 75
rules:
	- function_regex: pinky
		coverage: 20
		comment: nobody understands pinky
	- function_regex: the brain
		coverage: 90
		comment: the brain thinks he's understood
	- filename_regex: main.go
		coverage: 50
	- filename_regex: utils.go
		coverage: 95
`
	yml = strings.ReplaceAll(yml, "\t", "  ")
	config, err = parseYAMLConfig([]byte(yml))
	assert.Nil(t, err)
	assert.Equal(t, 75.0, config.DefaultCoverage)
	assert.Equal(t, 4, len(config.Rules))
	assert.Equal(t, "pinky", config.Rules[0].FunctionRegex)
}

func TestMakeFunctionLocationMapFailure(t *testing.T) {
	options := newTestOptions()
	options.dirToParse = "does-not-exist"
	_, err := makeFunctionLocationMap(options)
	assert.Error(t, err)
}

func TestMakeFunctionLocationMapSuccess(t *testing.T) {
	fmap, err := makeFunctionLocationMap(newTestOptions())
	assert.Nil(t, err)
	fls := []FunctionLocation{
		{
			Filename:   "functions-for-testing-makeFunctionLocationMap.go",
			LineNumber: "20",
			Function:   "functionAtLine20",
			Receiver:   "",
		},
		{
			Filename:   "functions-for-testing-makeFunctionLocationMap.go",
			LineNumber: "26",
			Function:   "String",
			Receiver:   "methodReceiver",
		},
	}
	for _, fl := range fls {
		key := fl.key()
		if assert.Contains(t, fmap, key) {
			assert.Equal(t, fl, fmap[key])
		}
	}
}

func TestMakeFunctionLocationMapSupport(t *testing.T) {
	assert.Equal(t, "This function is at line 20 to test makeFunctionLocationMap()", functionAtLine20())
	mr := methodReceiver{}
	assert.Equal(t, "This method has a methodReceiver receiver to test makeFunctionLocationMap()", mr.String())
}

func TestCaptureOutput(t *testing.T) {
	output, err := captureOutput("cat", "/non-existent")
	assert.Error(t, err)
	assert.Empty(t, output)
	assert.Contains(t, err.Error(), "cat: /non-existent: No such file or directory")

	output, err = captureOutput("cat", "/etc/passwd")
	assert.Nil(t, err)
	rootLines := []string{}
	for _, line := range output {
		if strings.HasPrefix(line, "root:") {
			rootLines = append(rootLines, line)
		}
	}
	assert.Len(t, rootLines, 1)
}

func TestGoCoverSuccess(t *testing.T) {
	fakeOutput := map[string][]string{
		"test --covermode set --coverprofile": {"ignored"},
		"tool cover --func":                   {"expected return value"},
	}
	commandRun := map[string]bool{}
	options := newTestOptions()
	options.captureOutput = func(command string, args ...string) ([]string, error) {
		// The random filename is always the last arg, so drop it.
		parts := args[0 : len(args)-1]
		key := strings.Join(parts, " ")
		commandRun[key] = true
		return fakeOutput[key], nil
	}
	actual, err := goCover(options)
	assert.Nil(t, err)
	assert.Equal(t, []string{"expected return value"}, actual)
	assert.Equal(t, len(commandRun), 2)
	assert.True(t, commandRun["test --covermode set --coverprofile"], commandRun)
	assert.True(t, commandRun["tool cover --func"], commandRun)
}

func TestGoCoverBrowserFailure(t *testing.T) {
	fakeOutput := map[string][]string{
		"test --covermode set --coverprofile": {"ignored"},
	}
	fakeErrors := map[string]error{
		"tool cover --html": fmt.Errorf("browser error"),
	}
	commandRun := map[string]bool{}
	options := newTestOptions()
	options.showCoverageInBrowser = true
	options.captureOutput = func(command string, args ...string) ([]string, error) {
		// The random filename is always the last arg, so drop it.
		parts := args[0 : len(args)-1]
		key := strings.Join(parts, " ")
		commandRun[key] = true
		return fakeOutput[key], fakeErrors[key]
	}

	actual, err := goCover(options)
	assert.Error(t, err)
	assert.Equal(t, []string{}, actual)
	assert.Equal(t, 2, len(commandRun), commandRun)
	assert.True(t, commandRun["test --covermode set --coverprofile"], commandRun)
	assert.True(t, commandRun["tool cover --html"], commandRun)
}

func TestGoCoverBrowser(t *testing.T) {
	fakeOutput := map[string][]string{
		"test --covermode set --coverprofile": {"ignored"},
		"tool cover --func":                   {"expected return value"},
		"tool cover --html":                   {"ignored"},
	}
	commandRun := map[string]bool{}
	options := newTestOptions()
	options.showCoverageInBrowser = true
	options.captureOutput = func(command string, args ...string) ([]string, error) {
		// The random filename is always the last arg, so drop it.
		parts := args[0 : len(args)-1]
		key := strings.Join(parts, " ")
		commandRun[key] = true
		return fakeOutput[key], nil
	}

	actual, err := goCover(options)
	assert.Nil(t, err)
	assert.Equal(t, []string{"expected return value"}, actual)
	assert.Equal(t, 3, len(commandRun), commandRun)
	assert.True(t, commandRun["test --covermode set --coverprofile"], commandRun)
	assert.True(t, commandRun["tool cover --func"], commandRun)
	assert.True(t, commandRun["tool cover --html"], commandRun)
}

func TestGoCoverCaptureFailure(t *testing.T) {
	options := newTestOptions()
	options.captureOutput = func(string, ...string) ([]string, error) {
		return []string{"this should not be seen"}, errors.New("error for testing")
	}
	actual, err := goCover(options)
	assert.Error(t, err)
	assert.Equal(t, []string{}, actual)
	assert.Contains(t, err.Error(), "error for testing")
}

func TestGoCoverCreateTempFailure(t *testing.T) {
	options := newTestOptions()
	options.createTemp = func(string, string) (*os.File, error) {
		return nil, errors.New("error for testing")
	}
	actual, err := goCover(options)
	assert.Error(t, err)
	assert.Equal(t, []string{}, actual)
	assert.Contains(t, err.Error(), "error for testing")
}

func validCoverageOutput() []string {
	coverage := `
github.com/tobinjt/golang-coverage-pre-commit/golang-coverage-pre-commit.go:26:		String			100.0%
github.com/tobinjt/golang-coverage-pre-commit/golang-coverage-pre-commit.go:48:		String			31.0%
github.com/tobinjt/golang-coverage-pre-commit/golang-coverage-pre-commit.go:53:		makeExampleConfig	50.0%
github.com/tobinjt/golang-coverage-pre-commit/golang-coverage-pre-commit.go:95:		parseYAMLConfig		100.0%
github.com/tobinjt/golang-coverage-pre-commit/golang-coverage-pre-commit.go:118:	realMain		17.3%
github.com/tobinjt/golang-coverage-pre-commit/golang-coverage-pre-commit.go:140:	main			0.0%
total:											(statements)		38.1%
`
	return strings.Split(coverage, "\n")
}

func TestParseCoverageOutputSuccess(t *testing.T) {
	options := newTestOptions()
	options.modulePath = "github.com/tobinjt/golang-coverage-pre-commit/"
	results, err := parseCoverageOutput(options, validCoverageOutput())
	assert.Nil(t, err)
	assert.Equal(t, 6, len(results))

	expected := []CoverageLine{
		{
			Filename:   "golang-coverage-pre-commit.go",
			LineNumber: "26",
			Function:   "String",
			Coverage:   100.0,
		},
		{
			Filename:   "golang-coverage-pre-commit.go",
			LineNumber: "48",
			Function:   "String",
			Coverage:   31.0,
		},
		{
			Filename:   "golang-coverage-pre-commit.go",
			LineNumber: "53",
			Function:   "makeExampleConfig",
			Coverage:   50.0,
		},
		{
			Filename:   "golang-coverage-pre-commit.go",
			LineNumber: "95",
			Function:   "parseYAMLConfig",
			Coverage:   100.0,
		},
		{
			Filename:   "golang-coverage-pre-commit.go",
			LineNumber: "118",
			Function:   "realMain",
			Coverage:   17.3,
		},
		{
			Filename:   "golang-coverage-pre-commit.go",
			LineNumber: "140",
			Function:   "main",
			Coverage:   0.0,
		},
	}
	assert.Equal(t, expected, results)
}

func TestParseCoverageOutputFailure(t *testing.T) {
	options := newTestOptions()

	badInputLine := `
github.com/.../golang-coverage-pre-commit.go:26:		String			100.0%
asdf
github.com/.../golang-coverage-pre-commit.go:140:	main			0.0%
total:											(statements)		38.1%
`
	_, err := parseCoverageOutput(options, strings.Split(badInputLine, "\n"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 3 parts, found 1")

	badInputLine = `missing-line-number:		String			100.0%`
	_, err = parseCoverageOutput(options, []string{badInputLine})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected `filename:linenumber:` in \"missing-line-number:\"")

	table := []struct {
		input string
		err   string
	}{
		{
			err:   "could not extract percentage from",
			input: "1.2",
		},
		{
			err:   "could not extract percentage from",
			input: "qwerty",
		},
		{
			err:   "strconv.ParseFloat: parsing",
			input: "asdf%",
		},
		{
			err:   "percentage (-12.2) < 0",
			input: "-12.2%",
		},
		{
			err:   "percentage (105.3) > 100",
			input: "105.3%",
		},
	}
	for _, test := range table {
		input := fmt.Sprintf("foo.go:26:		String			%s", test.input)
		_, err := parseCoverageOutput(options, []string{input})
		if assert.Error(t, err, test.input) {
			assert.Contains(t, err.Error(), test.err)
		}
	}
}

func stripComments(input []string) []string {
	output := []string{}
	for _, line := range input {
		if !strings.HasPrefix(line, "//") {
			output = append(output, line)
		}
	}
	return output
}

func TestCheckCoverage(t *testing.T) {
	tests := []struct {
		desc                string
		config              Config
		functionLocationMap map[string]FunctionLocation
		input               []string
		errors              []string
		debug               []string
	}{

		{
			desc: "Filename matching",
			config: Config{
				Rules: []Rule{
					{
						FilenameRegex: "^utils.go$",
						Coverage:      100,
					},
				},
			},
			input: []string{
				"// Matches, insufficient coverage.",
				"utils.go:1:	ReadFileOrDie	57.0%",
				"// Matches, sufficient coverage.",
				"utils.go:2:	ParseIntOrDie	100.0%",
				"// Doesn't match, falls through to default.",
				"main.go:1:	main	22.0%",
			},
			errors: []string{
				"utils.go:1:\tReadFileOrDie\t57.0%: actual coverage 57.0% < required coverage 100.0%: matching rule",
				"matching rule is `FilenameRegex: ^utils.go$ FunctionRegex:  ReceiverRegex:  Coverage: 100 Comment: `",
			},
			debug: []string{
				// First coverage line.
				"Debug info for coverage matching",
				"Line utils.go:1:\tReadFileOrDie\t57.0%\n",
				"Matching rule: FilenameRegex: ^utils.go$ FunctionRegex:  ReceiverRegex:  Coverage: 100",
				"actual coverage 57.0% < required coverage 100.0%",
				// Second coverage line.
				"Line utils.go:2:\tParseIntOrDie\t100.0%",
				"Matching rule: FilenameRegex: ^utils.go$ FunctionRegex:  ReceiverRegex:  Coverage: 100 Comment:",
				"actual coverage 100.0% >= required coverage 100.0%",
				// Third coverage line.
				"Line main.go:1:\tmain\t22.0%",
				"Default coverage 0.0% satisfied",
			},
		},

		{
			desc: "Function matching",
			config: Config{
				Rules: []Rule{
					{
						FunctionRegex: "OrDie$",
						Coverage:      100,
					},
				},
			},
			input: []string{
				"// Matches, insufficient coverage.",
				"utils.go:1:	ReadFileOrDie	57.0%",
				"// Matches, sufficient coverage.",
				"utils.go:2:	ParseIntOrDie	100.0%",
				"// Doesn't match, falls through to default.",
				"main.go:1:	main	100.0%",
			},
			errors: []string{
				"utils.go:1:\tReadFileOrDie\t57.0%: actual coverage 57.0% < required coverage 100.0%: matching rule",
				"matching rule is `FilenameRegex:  FunctionRegex: OrDie$ ReceiverRegex:  Coverage: 100 Comment: `",
			},
			debug: []string{
				// First coverage line.
				"Debug info for coverage matching",
				"Line utils.go:1:\tReadFileOrDie\t57.0%\n",
				"Matching rule: FilenameRegex:  FunctionRegex: OrDie$ ReceiverRegex:  Coverage: 100",
				"actual coverage 57.0% < required coverage 100.0%",
				// Second coverage line.
				"Line utils.go:2:\tParseIntOrDie\t100.0%",
				"Matching rule: FilenameRegex:  FunctionRegex: OrDie$ ReceiverRegex:  Coverage: 100 Comment:",
				"actual coverage 100.0% >= required coverage 100.0%",
				// Third coverage line.
				"Line main.go:1:\tmain\t100.0%",
				"Default coverage 0.0% satisfied",
			},
		},

		{
			desc: "Receiver matching",
			config: Config{
				Rules: []Rule{
					{
						ReceiverRegex: "^testReceiver$",
						Coverage:      100,
					},
				},
			},
			input: []string{
				"// Matches, insufficient coverage.",
				"utils.go:1:	Commit	57.0%",
				"// Matches, sufficient coverage.",
				"utils.go:2:	String	100.0%",
				"// Doesn't match, falls through to default.",
				"main.go:1:	main	100.0%",
			},
			functionLocationMap: map[string]FunctionLocation{
				"utils.go:1": {
					Filename:   "utils.go",
					LineNumber: "1",
					Function:   "Commit",
					Receiver:   "testReceiver",
				},
				"utils.go:2": {
					Filename:   "utils.go",
					LineNumber: "2",
					Function:   "String",
					Receiver:   "testReceiver",
				},
			},
			errors: []string{
				"utils.go:1:\tCommit\t57.0%: actual coverage 57.0% < required coverage 100.0%: matching rule",
				"matching rule is `FilenameRegex:  FunctionRegex:  ReceiverRegex: ^testReceiver$ Coverage: 100 Comment: `",
			},
			debug: []string{
				// First coverage line.
				"Debug info for coverage matching",
				"Line utils.go:1:\tCommit\t57.0%\n",
				"Matching rule: FilenameRegex:  FunctionRegex:  ReceiverRegex: ^testReceiver$ Coverage: 100",
				"actual coverage 57.0% < required coverage 100.0%",
				// Second coverage line.
				"Line utils.go:2:\tString\t100.0%",
				"Matching rule: FilenameRegex:  FunctionRegex:  ReceiverRegex: ^testReceiver$ Coverage: 100",
				"actual coverage 100.0% >= required coverage 100.0%",
				// Third coverage line.
				"Line main.go:1:\tmain\t100.0%",
				"Default coverage 0.0% satisfied",
			},
		},

		{
			desc: "Filename, Function, and Receiver matching",
			config: Config{
				Rules: []Rule{
					{
						FilenameRegex: "^utils.go$",
						FunctionRegex: "^Commit$",
						ReceiverRegex: "^testReceiver$",
						Coverage:      100,
					},
					{
						FilenameRegex: "^utils.go$",
						FunctionRegex: "^String$",
						ReceiverRegex: "^testReceiver$",
						Coverage:      100,
					},
				},
			},
			input: []string{
				"// Matches, insufficient coverage.",
				"utils.go:1:	Commit	57.0%",
				"// Matches, sufficient coverage.",
				"utils.go:2:	String	100.0%",
				"// Doesn't match, falls through to default.",
				"main.go:1:	main	100.0%",
			},
			functionLocationMap: map[string]FunctionLocation{
				"utils.go:1": {
					Filename:   "utils.go",
					LineNumber: "1",
					Function:   "Commit",
					Receiver:   "testReceiver",
				},
				"utils.go:2": {
					Filename:   "utils.go",
					LineNumber: "2",
					Function:   "String",
					Receiver:   "testReceiver",
				},
			},
			errors: []string{
				"utils.go:1:\tCommit\t57.0%: actual coverage 57.0% < required coverage 100.0%: matching rule",
				"matching rule is `FilenameRegex: ^utils.go$ FunctionRegex: ^Commit$ ReceiverRegex: ^testReceiver$ Coverage: 100",
			},
			debug: []string{
				// First coverage line.
				"Debug info for coverage matching",
				"Line utils.go:1:\tCommit\t57.0%\n",
				"Matching rule: FilenameRegex: ^utils.go$ FunctionRegex: ^Commit$ ReceiverRegex: ^testReceiver$ Coverage: 100",
				"actual coverage 57.0% < required coverage 100.0%",
				// Second coverage line.
				"Line utils.go:2:\tString\t100.0%",
				"Matching rule: FilenameRegex: ^utils.go$ FunctionRegex: ^String$ ReceiverRegex: ^testReceiver$ Coverage: 100",
				"actual coverage 100.0% >= required coverage 100.0%",
				// Third coverage line.
				"Line main.go:1:\tmain\t100.0%",
				"Default coverage 0.0% satisfied",
			},
		},

		{
			desc: "Default coverage",
			config: Config{
				DefaultCoverage: 90,
			},
			input: []string{
				"// Insufficient coverage.",
				"utils.go:1:	ReadFileOrDie	57.0%",
				"// Sufficient coverage.",
				"utils.go:2:	ParseIntOrDie	100.0%",
			},
			errors: []string{
				"utils.go:1:\tReadFileOrDie\t57.0%: actual coverage 57.0% < default coverage 90.0%",
			},
			debug: []string{
				// First coverage line.
				"Debug info for coverage matching",
				"Line utils.go:1:\tReadFileOrDie\t57.0%\n",
				"Default coverage 90.0% not satisfied",
				// Second coverage line.
				"Line utils.go:2:\tParseIntOrDie\t100.0%",
				"Default coverage 90.0% satisfied",
			},
		},

		{
			desc: "No errors",
			config: Config{
				DefaultCoverage: 90,
			},
			input: []string{
				"// Sufficient coverage.",
				"utils.go:2:	ParseIntOrDie	100.0%",
			},
			errors: []string{},
			debug: []string{
				// First coverage line.
				"Line utils.go:2:\tParseIntOrDie\t100.0%",
				"Default coverage 90.0% satisfied",
			},
		},
	}

	options := newTestOptions()
	for _, test := range tests {
		coverage, err := parseCoverageOutput(options, stripComments(test.input))
		assert.Nil(t, err)
		config, err := validateConfig(test.config)
		assert.Nil(t, err)

		debug, err := checkCoverage(config, coverage, test.functionLocationMap)
		if len(test.errors) == 0 {
			assert.Nil(t, err)
		} else {
			if assert.Error(t, err) {
				for i := range test.errors {
					assert.Contains(t, err.Error(), test.errors[i], "err: "+test.desc)
				}
			}
		}
		for i := range test.debug {
			assert.Contains(t, debug, test.debug[i], "debug: "+test.desc)
		}
	}
}

func TestRealMain(t *testing.T) {
	table := []struct {
		desc   string
		err    string
		output string
		mod    func(opts Options) Options
	}{
		{
			desc:   "makeExampleConfig",
			err:    "",
			output: "Comment is not interpreted or used",
			mod: func(opts Options) Options {
				opts.rawArgs = append(opts.rawArgs, "--example_config")
				return opts
			},
		},
		{
			desc:   "generateConfig",
			err:    "",
			output: "Generated rule for parseYAMLConfig",
			mod: func(opts Options) Options {
				opts.rawArgs = []string{"--generate_config"}
				opts.captureOutput = func(string, ...string) ([]string, error) {
					return validCoverageOutput(), nil
				}
				return opts
			},
		},
		{
			desc:   "bad go.mod path",
			err:    "failed reading go-mod-does-not-exist:",
			output: "",
			mod: func(opts Options) Options {
				opts.goMod = "go-mod-does-not-exist"
				return opts
			},
		},
		{
			desc:   "bad config path",
			err:    "failed reading config does-not-exist.yaml:",
			output: "",
			mod: func(opts Options) Options {
				opts.configFile = "does-not-exist.yaml"
				return opts
			},
		},
		{
			desc:   "bad config contents",
			err:    "failed parsing config bad-config.yaml:",
			output: "",
			mod: func(opts Options) Options {
				opts.configFile = "bad-config.yaml"
				return opts
			},
		},
		{
			desc:   "unexpected arguments",
			err:    "unexpected arguments",
			output: "",
			mod: func(opts Options) Options {
				opts.rawArgs = []string{"asdf", "1234"}
				return opts
			},
		},
		{
			desc:   "unsupported flag",
			err:    "flag provided but not defined: -bad-flag",
			output: "",
			mod: func(opts Options) Options {
				opts.rawArgs = []string{"--bad-flag"}
				opts.flagOutput = new(bytes.Buffer)
				return opts
			},
		},
		{
			desc:   "goCover fails",
			err:    "forced error for goCover fails",
			output: "",
			mod: func(opts Options) Options {
				opts.createTemp = func(_, __ string) (*os.File, error) {
					return nil, fmt.Errorf("forced error for goCover fails")
				}
				return opts
			},
		},
		{
			desc:   "parseCoverageOutput fails",
			err:    "expected 3 parts, found 1, in \"qwerty\"",
			output: "",
			mod: func(opts Options) Options {
				opts.captureOutput = func(string, ...string) ([]string, error) {
					return []string{"qwerty"}, nil
				}
				return opts
			},
		},
		// Note that from here on the failures are that coverage isn't high enough.
		{
			desc:   "checkCoverage",
			err:    "golang-coverage-pre-commit.go:48:\tString\t31.0%: actual coverage 31.0% < default coverage 100.0%",
			output: "",
			mod: func(opts Options) Options {
				opts.captureOutput = func(string, ...string) ([]string, error) {
					return validCoverageOutput(), nil
				}
				return opts
			},
		},
		{
			desc:   "checkCoverage, with debugging output",
			err:    "golang-coverage-pre-commit.go:48:\tString\t31.0%: actual coverage 31.0% < default coverage 100.0",
			output: "Debug info for coverage matching",
			mod: func(opts Options) Options {
				opts.rawArgs = append(opts.rawArgs, "--debug_matching")
				opts.captureOutput = func(string, ...string) ([]string, error) {
					return validCoverageOutput(), nil
				}
				return opts
			},
		},
	}

	for _, test := range table {
		options := test.mod(newTestOptions())
		output, err := realMain(options)
		if len(test.err) == 0 {
			assert.Nil(t, err, "error check for "+test.desc)
		} else {
			assert.Error(t, err, "error check for "+test.desc)
			assert.Contains(t, err.Error(), test.err, "error check for "+test.desc)
		}
		if len(test.output) > 0 {
			assert.Contains(t, output, test.output, "output check for "+test.desc)
		} else {
			assert.Equal(t, "", output, "output check for "+test.desc)
		}
	}
}

func TestRunAndPrintSuccess(t *testing.T) {
	options := newTestOptions()
	stdout := new(bytes.Buffer)
	options.stdout = stdout
	stderr := new(bytes.Buffer)
	options.stderr = stderr

	runMe := func(options Options) (string, error) {
		return "this is going to stdout", nil
	}
	runAndPrint(options, runMe)
	assert.Empty(t, stderr.String())
	assert.Contains(t, stdout.String(), "this is going to stdout")
}

func TestRunAndPrintErrors(t *testing.T) {
	options := newTestOptions()
	stdout := new(bytes.Buffer)
	options.stdout = stdout
	stderr := new(bytes.Buffer)
	options.stderr = stderr

	exitCalled := false
	options.exit = func(_ int) {
		exitCalled = true
	}

	runMe := func(options Options) (string, error) {
		return "this is going to stdout", fmt.Errorf("this is going to stderr")
	}
	runAndPrint(options, runMe)
	assert.True(t, exitCalled)
	assert.Contains(t, stderr.String(), "this is going to stderr")
	assert.Contains(t, stdout.String(), "this is going to stdout")
}
