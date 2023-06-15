// SPDX-License-Identifier: GPL-2.0-or-later

package devices

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/callbacks"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/clients"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/utils"
)

type PTPDeviceInfo struct {
	Timestamp string `json:"date" fetcherKey:"date"`
	VendorID  string `json:"vendorId" fetcherKey:"vendorID"`
	DeviceID  string `json:"deviceInfo" fetcherKey:"devID"`
	GNSSDev   string `json:"GNSSDev" fetcherKey:"gnss"`
}

// AnalyserJSON returns the json expected by the analysers
func (ptpDevInfo *PTPDeviceInfo) AnalyserJSON() ([]byte, error) {
	line, err := json.Marshal(&callbacks.AnalyserFormatType{
		ID: "devInfo",
		Data: []string{
			ptpDevInfo.Timestamp,
			ptpDevInfo.VendorID,
			ptpDevInfo.DeviceID,
			ptpDevInfo.GNSSDev,
		},
	})
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshal Analyser format for ptpDevInfo %w", err)
	}
	return line, nil
}

type DevDPLLInfo struct {
	Timestamp string `json:"date" fetcherKey:"date"`
	EECState  string `json:"EECState" fetcherKey:"dpll_0_state"`
	PPSState  string `json:"PPSState" fetcherKey:"dpll_1_state"`
	PPSOffset string `json:"PPSOffset" fetcherKey:"dpll_1_offset"`
}

// AnalyserJSON returns the json expected by the analysers
func (dpllInfo *DevDPLLInfo) AnalyserJSON() ([]byte, error) {
	offset, err := strconv.ParseFloat(dpllInfo.PPSOffset, 32)
	if err != nil {
		return []byte{}, fmt.Errorf("failed converting PPSOffset %w", err)
	}
	line, err := json.Marshal(&callbacks.AnalyserFormatType{
		ID: "dpll/time-error",
		Data: []string{
			dpllInfo.Timestamp,
			dpllInfo.EECState,
			dpllInfo.PPSState,
			fmt.Sprintf("%f", offset/unitConversionFactor),
		},
	})
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshal Analyser format for dpllInfo %w", err)
	}
	return line, nil
}

const (
	unitConversionFactor = 100
)

var (
	devFetcher  map[string]*fetcher
	dpllFetcher map[string]*fetcher
	dateCmd     *clients.Cmd
)

func init() {
	devFetcher = make(map[string]*fetcher)
	dpllFetcher = make(map[string]*fetcher)
	dateCmdInst, err := clients.NewCmd("date", "date +%s.%N")
	if err != nil {
		panic(err)
	}
	dateCmd = dateCmdInst
	dateCmd.SetCleanupFunc(FormatTimestampAsRFC3339Nano)
}
func FormatTimestampAsRFC3339Nano(s string) (string, error) {
	timestamp, err := utils.ParseTimestamp(strings.TrimSpace(s))
	if err != nil {
		return "", fmt.Errorf("failed to parse timestamp %w", err)
	}
	return timestamp.Format(time.RFC3339Nano), nil
}

// BuildPTPDeviceInfo popluates the fetcher required for
// collecting the PTPDeviceInfo
func BuildPTPDeviceInfo(interfaceName string) error {
	fetcherInst := NewFetcher()
	devFetcher[interfaceName] = fetcherInst

	fetcherInst.AddCommand(dateCmd)

	err := fetcherInst.AddNewCommand(
		"gnss",
		fmt.Sprintf("ls /sys/class/net/%s/device/gnss/", interfaceName),
		true,
	)
	if err != nil {
		log.Errorf("failed to add command %s %s", "gnss", err.Error())
		return fmt.Errorf("failed to fetch devInfo %w", err)
	}

	err = fetcherInst.AddNewCommand(
		"devID",
		fmt.Sprintf("cat /sys/class/net/%s/device/device", interfaceName),
		true,
	)
	if err != nil {
		log.Errorf("failed to add command %s %s", "devId", err.Error())
		return fmt.Errorf("failed to fetch devInfo %w", err)
	}
	err = fetcherInst.AddNewCommand("vendorID",
		fmt.Sprintf("cat /sys/class/net/%s/device/vendor", interfaceName),
		true,
	)
	if err != nil {
		log.Errorf("failed to add command %s %s", "vendorID", err.Error())
		return fmt.Errorf("failed to fetch devInfo %w", err)
	}
	return nil
}

// GetPTPDeviceInfo returns the PTPDeviceInfo for an interface
func GetPTPDeviceInfo(interfaceName string, ctx clients.ContainerContext) (PTPDeviceInfo, error) {
	devInfo := PTPDeviceInfo{}
	// Find the dev for the GNSS for this interface
	fetcherInst, fetchedInstanceOk := devFetcher[interfaceName]
	if !fetchedInstanceOk {
		err := BuildPTPDeviceInfo(interfaceName)
		if err != nil {
			return devInfo, err
		}
		fetcherInst, fetchedInstanceOk = devFetcher[interfaceName]
		if !fetchedInstanceOk {
			return devInfo, errors.New("failed to create fetcher for PTPDeviceInfo")
		}
	}

	err := fetcherInst.Fetch(ctx, &devInfo)
	if err != nil {
		log.Errorf("failed to fetch devInfo %s", err.Error())
		return devInfo, fmt.Errorf("failed to fetch devInfo %w", err)
	}
	devInfo.GNSSDev = "/dev/" + devInfo.GNSSDev
	return devInfo, nil
}

// BuildDPLLInfoFetcher popluates the fetcher required for
// collecting the DPLLInfo
func BuildDPLLInfoFetcher(interfaceName string) error {
	fetcherInst := NewFetcher()
	dpllFetcher[interfaceName] = fetcherInst

	fetcherInst.AddCommand(dateCmd)

	err := fetcherInst.AddNewCommand(
		"dpll_0_state",
		fmt.Sprintf("cat /sys/class/net/%s/device/dpll_0_state", interfaceName),
		true,
	)
	if err != nil {
		log.Errorf("failed to add command %s %s", "dpll_0_state", err.Error())
		return err
	}

	err = fetcherInst.AddNewCommand(
		"dpll_1_state",
		fmt.Sprintf("cat /sys/class/net/%s/device/dpll_1_state", interfaceName),
		true,
	)
	if err != nil {
		log.Errorf("failed to add command %s %s", "dpll_1_state", err.Error())
		return err
	}

	err = fetcherInst.AddNewCommand(
		"dpll_1_offset",
		fmt.Sprintf("cat /sys/class/net/%s/device/dpll_1_offset", interfaceName),
		true,
	)
	if err != nil {
		log.Errorf("failed to add command %s %s", "dpll_1_offset", err.Error())
		return err
	}
	return nil
}

// GetDevDPLLInfo returns the device DPLL info for an interface.
func GetDevDPLLInfo(ctx clients.ContainerContext, interfaceName string) (DevDPLLInfo, error) {
	dpllInfo := DevDPLLInfo{}
	fetcherInst, fetchedInstanceOk := dpllFetcher[interfaceName]
	if !fetchedInstanceOk {
		err := BuildDPLLInfoFetcher(interfaceName)
		if err != nil {
			return dpllInfo, err
		}
		fetcherInst, fetchedInstanceOk = dpllFetcher[interfaceName]
		if !fetchedInstanceOk {
			return dpllInfo, errors.New("failed to create fetcher for DPLLInfo")
		}
	}
	err := fetcherInst.Fetch(ctx, &dpllInfo)
	if err != nil {
		log.Errorf("failed to fetch dpllInfo %s", err.Error())
		return dpllInfo, err
	}
	return dpllInfo, nil
}
