//go:build windows

package main

import (
	"fmt"

	"github.com/jaypipes/ghw"
)
var (
	driveMap = make(map[string]*ghw.Disk)
	partitionMap = make(map[string]*ghw.Partition)
)

func List_Drives() []string  {
	block, _ := ghw.Block()
	drives := []string{}
    for _, d := range block.Disks {
		driveMap[d.Model] = d
		drives = append(drives, d.Model)
	}
	return drives
}

func List_Partitions() []string {
	block, _ := ghw.Block()
	paritions := []string{}
	for _, d := range block.Disks {
		for _, p := range d.Partitions {
			paritions = append(paritions, fmt.Sprintf("%s %s", p.Name, d.Model))
			partitionMap[p.Name] = p
		}
	}
	return paritions
}
