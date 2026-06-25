package collector

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type CPUStat struct {
	Total uint64
	Idle  uint64
}

type MemoryMetrics struct {
	UsedPercent    float64
	AvailableBytes uint64
}

type FilesystemMetrics struct {
	UsedPercent float64
	TotalBytes  uint64
	UsedBytes   uint64
}

type DiskMetrics struct {
	ReadBytes  uint64
	WriteBytes uint64
}

type NetworkMetrics struct {
	RXBytes uint64
	TXBytes uint64
}

type AgentMetrics struct {
	RSSBytes uint64
}

type Snapshot struct {
	CPU        CPUStat
	Memory     MemoryMetrics
	Filesystem FilesystemMetrics
	Disk       DiskMetrics
	Network    NetworkMetrics
	Agent      AgentMetrics
}

func Collect(rootPath string, networkExcludePrefixes, diskExcludePrefixes []string) (Snapshot, error) {
	cpuBody, err := os.ReadFile("/proc/stat")
	if err != nil {
		return Snapshot{}, err
	}
	cpu, err := ParseCPUStat(string(cpuBody))
	if err != nil {
		return Snapshot{}, err
	}
	memFile, err := os.Open("/proc/meminfo")
	if err != nil {
		return Snapshot{}, err
	}
	mem, memErr := ParseMeminfo(memFile)
	closeErr := memFile.Close()
	if memErr != nil {
		return Snapshot{}, memErr
	}
	if closeErr != nil {
		return Snapshot{}, closeErr
	}
	fs, err := CollectFilesystem(rootPath)
	if err != nil {
		return Snapshot{}, err
	}
	diskFile, err := os.Open("/proc/diskstats")
	if err != nil {
		return Snapshot{}, err
	}
	disk, diskErr := ParseDiskstats(diskFile, diskExcludePrefixes, 512)
	closeErr = diskFile.Close()
	if diskErr != nil {
		return Snapshot{}, diskErr
	}
	if closeErr != nil {
		return Snapshot{}, closeErr
	}
	netFile, err := os.Open("/proc/net/dev")
	if err != nil {
		return Snapshot{}, err
	}
	netMetrics, netErr := ParseNetDev(netFile, networkExcludePrefixes)
	closeErr = netFile.Close()
	if netErr != nil {
		return Snapshot{}, netErr
	}
	if closeErr != nil {
		return Snapshot{}, closeErr
	}
	return Snapshot{
		CPU:        cpu,
		Memory:     mem,
		Filesystem: fs,
		Disk:       disk,
		Network:    netMetrics,
		Agent:      CollectAgent(),
	}, nil
}

func ParseCPUStat(body string) (CPUStat, error) {
	scanner := bufio.NewScanner(strings.NewReader(body))
	if !scanner.Scan() {
		return CPUStat{}, errors.New("empty /proc/stat")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return CPUStat{}, errors.New("cpu aggregate row missing")
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return CPUStat{}, fmt.Errorf("parse cpu field %q: %w", field, err)
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return CPUStat{Total: total, Idle: idle}, nil
}

func CPUUsagePercent(prev, next CPUStat) float64 {
	totalDelta := next.Total - prev.Total
	idleDelta := next.Idle - prev.Idle
	if totalDelta == 0 || idleDelta > totalDelta {
		return 0
	}
	return float64(totalDelta-idleDelta) / float64(totalDelta) * 100
}

func ParseMeminfo(r io.Reader) (MemoryMetrics, error) {
	values := map[string]uint64{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return MemoryMetrics{}, err
		}
		values[key] = value * 1024
	}
	if err := scanner.Err(); err != nil {
		return MemoryMetrics{}, err
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 || available > total {
		return MemoryMetrics{}, errors.New("invalid meminfo totals")
	}
	return MemoryMetrics{
		AvailableBytes: available,
		UsedPercent:    float64(total-available) / float64(total) * 100,
	}, nil
}

func CollectFilesystem(path string) (FilesystemMetrics, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return FilesystemMetrics{}, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total == 0 || free > total {
		return FilesystemMetrics{}, errors.New("invalid filesystem totals")
	}
	used := total - free
	return FilesystemMetrics{
		TotalBytes:  total,
		UsedBytes:   used,
		UsedPercent: float64(used) / float64(total) * 100,
	}, nil
}

func ParseDiskstats(r io.Reader, excludePrefixes []string, sectorSize uint64) (DiskMetrics, error) {
	var metrics DiskMetrics
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		name := fields[2]
		if hasAnyPrefix(name, excludePrefixes) {
			continue
		}
		readSectors, err := strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			return DiskMetrics{}, err
		}
		writeSectors, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return DiskMetrics{}, err
		}
		metrics.ReadBytes += readSectors * sectorSize
		metrics.WriteBytes += writeSectors * sectorSize
	}
	if err := scanner.Err(); err != nil {
		return DiskMetrics{}, err
	}
	return metrics, nil
}

func ParseNetDev(r io.Reader, excludePrefixes []string) (NetworkMetrics, error) {
	var metrics NetworkMetrics
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		if hasAnyPrefix(name, excludePrefixes) {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		rx, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return NetworkMetrics{}, err
		}
		tx, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return NetworkMetrics{}, err
		}
		metrics.RXBytes += rx
		metrics.TXBytes += tx
	}
	if err := scanner.Err(); err != nil {
		return NetworkMetrics{}, err
	}
	return metrics, nil
}

func CollectAgent() AgentMetrics {
	file, err := os.Open("/proc/self/statm")
	if err != nil {
		return AgentMetrics{}
	}
	defer file.Close()
	rss, err := ParseStatmRSSBytes(file, uint64(os.Getpagesize()))
	if err != nil {
		return AgentMetrics{}
	}
	return AgentMetrics{RSSBytes: rss}
}

func ParseStatmRSSBytes(r io.Reader, pageSize uint64) (uint64, error) {
	body, err := io.ReadAll(io.LimitReader(r, 4096))
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(body))
	if len(fields) < 2 {
		return 0, errors.New("statm resident field missing")
	}
	residentPages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return residentPages * pageSize, nil
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
