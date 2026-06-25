package collector

import (
	"strings"
	"testing"
)

func TestCPUUsagePercentFromStatDeltas(t *testing.T) {
	prev, err := ParseCPUStat("cpu  100 0 50 850 0 0 0 0 0 0\n")
	if err != nil {
		t.Fatal(err)
	}
	next, err := ParseCPUStat("cpu  130 0 70 900 0 0 0 0 0 0\n")
	if err != nil {
		t.Fatal(err)
	}

	got := CPUUsagePercent(prev, next)
	if got < 49.9 || got > 50.1 {
		t.Fatalf("CPUUsagePercent = %v, want about 50", got)
	}
}

func TestParseMeminfoUsesMemAvailable(t *testing.T) {
	metrics, err := ParseMeminfo(strings.NewReader("MemTotal:       1000 kB\nMemFree:         100 kB\nMemAvailable:    250 kB\nBuffers:          10 kB\n"))
	if err != nil {
		t.Fatal(err)
	}

	if metrics.AvailableBytes != 250*1024 {
		t.Fatalf("AvailableBytes = %d", metrics.AvailableBytes)
	}
	if metrics.UsedPercent < 74.9 || metrics.UsedPercent > 75.1 {
		t.Fatalf("UsedPercent = %v, want about 75", metrics.UsedPercent)
	}
}

func TestParseNetDevExcludesNoisyInterfaces(t *testing.T) {
	body := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 100 1 0 0 0 0 0 0 200 1 0 0 0 0 0 0
  eth0: 300 1 0 0 0 0 0 0 400 1 0 0 0 0 0 0
 veth0: 900 1 0 0 0 0 0 0 900 1 0 0 0 0 0 0
`
	metrics, err := ParseNetDev(strings.NewReader(body), []string{"lo", "veth"})
	if err != nil {
		t.Fatal(err)
	}

	if metrics.RXBytes != 300 || metrics.TXBytes != 400 {
		t.Fatalf("metrics = %#v, want eth0 bytes only", metrics)
	}
}

func TestParseDiskstatsAggregatesSectors(t *testing.T) {
	body := "   8       0 sda 10 0 20 0 5 0 7 0 0 0 0 0 0 0 0 0 0\n   7       0 loop0 10 0 99 0 5 0 99 0 0 0 0 0 0 0 0 0 0\n"
	metrics, err := ParseDiskstats(strings.NewReader(body), []string{"loop", "ram"}, 512)
	if err != nil {
		t.Fatal(err)
	}

	if metrics.ReadBytes != 20*512 {
		t.Fatalf("ReadBytes = %d", metrics.ReadBytes)
	}
	if metrics.WriteBytes != 7*512 {
		t.Fatalf("WriteBytes = %d", metrics.WriteBytes)
	}
}

func TestCollectFilesystemReportsUsedPercent(t *testing.T) {
	metrics, err := CollectFilesystem(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if metrics.TotalBytes == 0 {
		t.Fatal("TotalBytes should be greater than zero")
	}
	if metrics.UsedPercent < 0 || metrics.UsedPercent > 100 {
		t.Fatalf("UsedPercent = %v, want 0..100", metrics.UsedPercent)
	}
}

func TestParseStatmRSSBytes(t *testing.T) {
	got, err := ParseStatmRSSBytes(strings.NewReader("100 25 0 0 0 0 0\n"), 4096)
	if err != nil {
		t.Fatal(err)
	}
	if got != 25*4096 {
		t.Fatalf("RSS bytes = %d, want %d", got, 25*4096)
	}
}
