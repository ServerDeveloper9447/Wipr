package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jaypipes/ghw"
)

type Config struct {
	MinimizeOnClose bool
	EnterpriseMode  bool
	PassKey         string
}

func ternary[T any](cond bool, iftrue T, iffalse T) T {
	if cond {
		return iftrue
	}
	return iffalse
}

const (
	WIDTH  = 700
	HEIGHT = 400
)

var (
	isWiping = false
	config   Config
	driveMap     = make(map[string]*ghw.Disk)
	partitionMap = make(map[string]*ghw.Partition)
)

func List_Drives() []string {
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
			partitionMap[fmt.Sprintf("%s %s", p.Name, d.Model)] = p
		}
	}
	return paritions
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func shortenPath(path string) (string, error) {
	cleanPath := filepath.ToSlash(path)

	dir, file := filepath.Split(cleanPath)

	if file == "" {
		return "", fmt.Errorf("invalid path: no file name found")
	}

	dirParts := strings.Split(strings.TrimSuffix(dir, "/"), "/")

	var shortDir []string
	if len(dirParts) > 3 {
		shortDir = append(shortDir, dirParts[0], "...")

		for i := len(dirParts) - 2; i < len(dirParts); i++ {
			folderName := dirParts[i]
			if len(folderName) > 10 {
				shortDir = append(shortDir, folderName[:5]+"[...]"+folderName[len(folderName)-5:])
			} else {
				shortDir = append(shortDir, folderName)
			}
		}
	} else {
		shortDir = dirParts
	}

	shortenedDir := strings.Join(shortDir, "/") + "/"

	ext := filepath.Ext(file)
	name := strings.TrimSuffix(file, ext)
	var shortFile string
	if len(name) > 10 {
		shortFile = name[:5] + "[...]" + name[len(name)-5:] + ext
	} else {
		shortFile = file
	}

	return shortenedDir + shortFile, nil
}

func init() {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		fmt.Println("Unsupported OS")
		return
	}
	setup_creds()
}

func main() {
	Wipr()
}
