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
	IsSSD      bool
	Partitions []Win32_PartitionWithLabel
}

type MSFT_PhysicalDisk struct {
	FriendlyName string
	SerialNumber string
	MediaType    uint16 // 3 = HDD, 4 = SSD
}

func List_Drives() []Win32_Disk {
	var diskDrives []Win32_Disk
	var drives []Win32_DiskDrive
	var partitions []Win32_DiskPartition
	var mappings []Win32_LogicalDiskToPartition
	var logicalDisks []Win32_LogicalDisk
	var physicalDisks []MSFT_PhysicalDisk

	if err := wmi.Query("SELECT DeviceID, Model, SerialNumber, Size, Index FROM Win32_DiskDrive WHERE MediaType='Fixed hard disk media'", &drives); err != nil {
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
	if err := wmi.QueryNamespace("SELECT FriendlyName, SerialNumber, MediaType FROM MSFT_PhysicalDisk", &physicalDisks, "ROOT\\Microsoft\\Windows\\Storage"); err != nil {
		log.Fatal(err)
	}

	for _, d := range drives {
		isSSD := false
		for _, pd := range physicalDisks {
			if strings.EqualFold(strings.TrimSpace(pd.SerialNumber), strings.TrimSpace(d.SerialNumber)) {
				isSSD = pd.MediaType == 4
				break
			}
		}

		drive := Win32_Disk{
			Drive:      d,
			IsSSD:      isSSD,
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
