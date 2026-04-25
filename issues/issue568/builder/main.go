package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	winswExampleConfig := `
<service>
    <id>{{issue}}</id>
    <name>{{issue}}</name>
    <description>{{issue}}</description>

    <executable>{{executable}}</executable>
    <workingdirectory>{{workingdirectory}}</workingdirectory>
    <stoptimeout>30 sec</stoptimeout>

    <delayedAutoStart>true</delayedAutoStart>
    <onfailure action="restart" delay="5000" />
	<arguments>"0 0 0 * * *"</arguments>
</service>
`
	winswConfigPath := "../winsw.xml"

	var issue string
	{
		abs, err := filepath.Abs("../")
		if err != nil {
			panic(err)
		}
		issue = filepath.Base(abs)
	}
	execPath := fmt.Sprintf("../%s.exe", issue)
	execAbsPath, err := filepath.Abs(execPath)
	if err != nil {
		panic(err)
	}

	_ = os.Remove(execPath)
	output, err := exec.Command("go", "build", "-o", execPath, "../").CombinedOutput()
	if err != nil {
		fmt.Printf("output: `%s`\n", string(output))
		panic(err)
	}

	winswConfig := winswExampleConfig
	winswConfig = strings.ReplaceAll(winswConfig, "{{issue}}", issue)
	winswConfig = strings.ReplaceAll(winswConfig, "{{executable}}", execAbsPath)
	winswConfig = strings.ReplaceAll(winswConfig, "{{workingdirectory}}", filepath.Dir(execAbsPath))
	err = os.WriteFile(winswConfigPath, []byte(winswConfig), os.ModePerm)
	if err != nil {
		panic(err)
	}
}
