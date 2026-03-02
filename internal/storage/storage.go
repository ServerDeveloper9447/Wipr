package storage

import (
	"github.com/jaypipes/ghw"
)

var (
	DriveMap     = make(map[string]*ghw.Disk)
	PartitionMap = make(map[string]*ghw.Partition)
)

func List_Drives() []string {
	block, _ := ghw.Block()
	drives := []string{}
	for _, d := range block.Disks {
		DriveMap[d.Model] = d
		drives = append(drives, d.Model)
	}
	return drives
}
