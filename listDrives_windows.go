//go:build windows

package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/StackExchange/wmi"
)

type Win32_DiskDrive struct {
	DeviceID     string
	Model        string
	SerialNumber string
	Size         uint64
	Index        uint32
}

type Win32_DiskPartition struct {
	DeviceID  string
	DiskIndex uint32
	Size      uint64
	Name      string
}

type Win32_LogicalDiskToPartition struct {
	Antecedent string
	Dependent  string
}

type Win32_LogicalDisk struct {
	DeviceID   string
	VolumeName string
}

type Win32_PartitionWithLabel struct {
	DeviceID  string
	DiskIndex uint32
	Size      uint64
	Name      string
	Letter    string
	Label     string
}

type Win32_Disk struct {
	Drive      Win32_DiskDrive
	Partitions []Win32_PartitionWithLabel
}

func List_Drives() []Win32_Disk {
	var diskDrives []Win32_Disk
	var drives []Win32_DiskDrive
	var partitions []Win32_DiskPartition
	var mappings []Win32_LogicalDiskToPartition
	var logicalDisks []Win32_LogicalDisk

	if err := wmi.Query("SELECT DeviceID, Model, SerialNumber, Size, Index FROM Win32_DiskDrive", &drives); err != nil {
		log.Fatal(err)
	}
	if err := wmi.Query("SELECT DeviceID, DiskIndex, Size, Name FROM Win32_DiskPartition WHERE Size >= 1073741824", &partitions); err != nil {
		log.Fatal(err)
	}
	if err := wmi.Query("SELECT Antecedent, Dependent FROM Win32_LogicalDiskToPartition", &mappings); err != nil {
		log.Fatal(err)
	}
	if err := wmi.Query("SELECT DeviceID, VolumeName FROM Win32_LogicalDisk", &logicalDisks); err != nil {
		log.Fatal(err)
	}

	for _, d := range drives {
		drive := Win32_Disk{
			Drive:      d,
			Partitions: []Win32_PartitionWithLabel{},
		}
		for _, p := range partitions {
			if p.DiskIndex == d.Index {
				for _, m := range mappings {
					if strings.Contains(m.Antecedent, p.DeviceID) {
						letter := extractDriveLetter(m.Dependent)
						var label string
						for _, v := range logicalDisks {
							if v.DeviceID == letter {
								label = v.VolumeName
							}
						}
						drive.Partitions = append(drive.Partitions, Win32_PartitionWithLabel{
							p.DeviceID,
							p.DiskIndex,
							p.Size,
							p.Name,
							letter,
							label,
						})
					}
				}
			}
		}
		diskDrives = append(diskDrives, drive)
	}

	return diskDrives
}

func extractDriveLetter(dependent string) string {
	dependent = dependent[strings.Index(dependent, "Win32_LogicalDisk.DeviceID="):]
	var letter string
	fmt.Sscanf(dependent, "Win32_LogicalDisk.DeviceID=\"%s", &letter)
	if len(letter) > 0 && letter[len(letter)-1] == '"' {
		letter = letter[:len(letter)-1]
	}
	return letter
}
