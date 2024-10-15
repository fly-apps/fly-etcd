package flycheck

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"syscall"

	"github.com/superfly/fly-checks/check"
)

// CheckVM for system / disk checks
func checkVM(_ context.Context, checks *check.CheckSuite) (*check.CheckSuite, error) {
	checks.AddCheck("disk", func() (string, error) {
		return checkDisk("/etcd_data/")
	})
	checks.AddCheck("load", func() (string, error) {
		return checkLoad()
	})
	checks.AddCheck("pressure", func() (string, error) {
		return checkPressure("memory")
	})

	return checks, nil
}

// checkPressure is an informational check that reports on the system's pressure
// It will only return an error if we can't read the pressure file.
func checkPressure(name string) (string, error) {
	var avg10, avg60, avg300, counter float64

	raw, err := os.ReadFile("/proc/pressure/" + name)
	if err != nil {
		return "", err
	}

	_, err = fmt.Sscanf(
		string(raw),
		"some avg10=%f avg60=%f avg300=%f total=%f",
		&avg10, &avg60, &avg300, &counter,
	)

	if err != nil {
		return "", err
	}

	if avg10 > 5 {
		return fmt.Sprintf("system spent %.1f of the last 10 seconds waiting for %s", avg10, name), nil
	}

	if avg60 > 10 {
		return fmt.Sprintf("system spent %.1f of the last 60 seconds waiting for %s", avg60, name), nil
	}

	if avg300 > 50 {
		return fmt.Sprintf("system spent %.1f of the last 300 seconds waiting for %s", avg300, name), nil
	}

	return fmt.Sprintf("%s: %.1fs waiting over the last 60s", name, avg60), nil
}

// checkLoad is an informational check that reports on the system's load averages
func checkLoad() (string, error) {
	var loadAverage1, loadAverage5, loadAverage10 float64
	var runningProcesses, totalProcesses, lastProcessID int
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "", err
	}

	cpus := float64(runtime.NumCPU())
	_, err = fmt.Sscanf(string(raw), "%f %f %f %d/%d %d",
		&loadAverage1, &loadAverage5, &loadAverage10,
		&runningProcesses, &totalProcesses,
		&lastProcessID)
	if err != nil {
		return "", err
	}

	if loadAverage1/cpus > 10 {
		return fmt.Sprintf("1 minute load average is very high: %.2f", loadAverage1), nil
	}
	if loadAverage5/cpus > 4 {
		return fmt.Sprintf("5 minute load average is high: %.2f", loadAverage5), nil
	}
	if loadAverage10/cpus > 2 {
		return fmt.Sprintf("10 minute load average is high: %.2f", loadAverage10), nil
	}

	return fmt.Sprintf("load averages: %.2f %.2f %.2f", loadAverage10, loadAverage5, loadAverage1), nil
}

// checkDisk is a failure check that reports on the disk space available
func checkDisk(dir string) (string, error) {
	var stat syscall.Statfs_t

	err := syscall.Statfs(dir, &stat)

	if err != nil {
		return "", fmt.Errorf("%s: %s", dir, err)
	}

	// Available blocks * size per block = available space in bytes
	size := stat.Blocks * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	pct := float64(available) / float64(size)
	msg := fmt.Sprintf("%s (%.1f%%) free space on %s", dataSize(available), pct*100, dir)

	if pct < 0.1 {
		return "", errors.New(msg)
	}

	return msg, nil
}

func round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}
func dataSize(size uint64) string {
	var suffixes [5]string
	suffixes[0] = "B"
	suffixes[1] = "KB"
	suffixes[2] = "MB"
	suffixes[3] = "GB"
	suffixes[4] = "TB"

	base := math.Log(float64(size)) / math.Log(1024)
	getSize := round(math.Pow(1024, base-math.Floor(base)), .5, 2)
	getSuffix := suffixes[int(math.Floor(base))]
	return fmt.Sprint(strconv.FormatFloat(getSize, 'f', -1, 64) + " " + string(getSuffix))
}
