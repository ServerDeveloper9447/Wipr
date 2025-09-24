//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/AlecAivazis/survey/v2"
	"github.com/jaypipes/ghw"
)

type Data struct {
	Mode string
	Path string
}

func setup_creds() {}

func ElevateOnLaunch() {
	if os.Geteuid() != 0 {
		exe, _ := os.Executable()
		args := os.Args[1:]
		cmd := exec.Command("pkexec", append([]string{exe}, args...)...)
		if err := cmd.Start(); err == nil {
			fmt.Println(err)
			return
		}
		cmd = exec.Command("sudo", append([]string{exe}, args...)...)
		cmd.Start()
		return
	}
}

func Wipr() {
	ElevateOnLaunch()
	block, err := ghw.Block()
	if err != nil {
		panic(err)
	}
	for _, d := range block.Disks {
		driveMap[d.Model] = d
		for _, p := range d.Partitions {
			partitionMap[fmt.Sprintf("%s %s", p.Name, d.Model)] = p
		}
	}
	var mode *string
	survey.AskOne(&survey.Select{
		Message: "Select a Mode:",
		Options: []string{"By Partitions", "By Disk Drive"},
	}, &mode)
	if mode != nil {
		switch *mode {
		case "By Partitions":
			var p *string
			survey.AskOne(&survey.Select{
				Message: "Select a Partition:",
				Options: List_Partitions(),
			}, &p)
			partition := partitionMap[*p]
			fmt.Println(partition)
		case "By Disk Drive":
			var d *string
			survey.AskOne(&survey.Select{
				Message: "Select a Drive",
				Options: List_Drives(),
			}, &d)
			drive := driveMap[*d]
			fmt.Println(drive)
		}
	}
}
