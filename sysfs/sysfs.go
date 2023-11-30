// Copyright 2022 IronCore authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sysfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironcore-dev/metalnet/encoding/sysfs"
	"github.com/jaypipes/ghw"
)

const DefaultMountPoint = "/sys"

type FS string

func NewDefaultFS() (FS, error) {
	return NewFS(DefaultMountPoint)
}

func NewFS(mountPoint string) (FS, error) {
	stat, err := os.Stat(mountPoint)
	if err != nil {
		return "", fmt.Errorf("error stat-ing %s: %w", mountPoint, err)
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("mount point %s is not a directory", mountPoint)
	}
	return FS(mountPoint), nil
}

func (fs FS) Path(segments ...string) string {
	return filepath.Join(append([]string{string(fs)}, segments...)...)
}

func (fs FS) PCIDevicePath(segments ...string) string {
	return fs.Path(append([]string{"bus", "pci", "devices"}, segments...)...)
}

func (fs FS) PCIDevices() ([]PCIDevice, error) {
	entries, err := os.ReadDir(fs.PCIDevicePath())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, nil
	}

	var devices []PCIDevice
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		address := ghw.PCIAddressFromString(entry.Name())
		if address == nil {
			continue
		}

		devices = append(devices, PCIDevice(filepath.Join(fs.PCIDevicePath(), entry.Name())))
	}
	return devices, nil
}

func (fs FS) PCIDevice(address ghw.PCIAddress) (PCIDevice, error) {
	p := fs.PCIDevicePath(address.String())
	stat, err := os.Stat(p)
	if err != nil {
		return "", err
	}

	if !stat.IsDir() {
		return "", fmt.Errorf("pci device %s is not a directory", p)
	}

	return PCIDevice(p), nil
}

type SRIOV struct {
	NumVFs   uint64 `sysfs:"sriov_numvfs"`
	TotalVFs uint64 `sysfs:"sriov_totalvfs"`
	Offset   uint64 `sysfs:"sriov_offset"`
	Stride   uint64 `sysfs:"sriov_stride"`
}

type PCIDevice string

func (p PCIDevice) SRIOV() (*SRIOV, error) {
	sriov := &SRIOV{}
	if err := sysfs.Unmarshal(string(p), sriov); err != nil {
		return nil, err
	}
	return sriov, nil
}

func (p PCIDevice) Virtfns() ([]PCIDevice, error) {
	virtfnPaths, err := filepath.Glob(filepath.Join(string(p), "virtfn[0-9]*"))
	if err != nil {
		return nil, err
	}

	virtfns := make([]PCIDevice, len(virtfnPaths))
	for i, virtfn := range virtfnPaths {
		virtfns[i] = PCIDevice(virtfn)
	}
	return virtfns, nil
}

func (p PCIDevice) Physfn() (PCIDevice, error) {
	physfnPath := filepath.Join(string(p), "physfn")
	stat, err := os.Stat(physfnPath)
	if err != nil {
		return "", err
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("physfn %s is not a directory", physfnPath)
	}
	return PCIDevice(physfnPath), nil
}

func (p PCIDevice) Address() (*ghw.PCIAddress, error) {
	path, err := filepath.EvalSymlinks(string(p))
	if err != nil {
		return nil, err
	}

	name := filepath.Base(path)
	res := ghw.PCIAddressFromString(name)
	if res == nil {
		return nil, fmt.Errorf("invalid pci address name %q", name)
	}
	return res, nil
}
