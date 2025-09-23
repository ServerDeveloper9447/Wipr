package main

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/jaypipes/ghw"
)

var (
	DriveMap    = make(map[string]*ghw.Disk)
	ParitionMap = make(map[string]*ghw.Partition)
)

func Wipr_Cli() {
	block, err := ghw.Block()
	if err != nil {
		panic(err)
	}
	for _, d := range block.Disks {
		DriveMap[d.Model] = d
		for _, p := range d.Partitions {
			ParitionMap[fmt.Sprintf("%s %s", p.Name, d.Model)] = p
		}
	}
	var mode *string
	survey.AskOne(&survey.Select{
		Message: "Select a Mode:",
		Options: []string{"By Partitions", "By Disk Drive"},
	}, &mode)
	switch *mode {
	case "By Partitions":
		var p *string
		survey.AskOne(&survey.Select{
			Message: "Select a Partition:",
			Options: List_Partitions(),
		}, &p)
		// partition := ParitionMap[*p]
		fmt.Println("Ctrl + C to cancel")

	}
}
