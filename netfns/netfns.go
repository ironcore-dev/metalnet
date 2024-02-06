// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package netfns

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ironcore-dev/metalnet/sysfs"
	"github.com/jaypipes/ghw"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	perm     = 0777
	filePerm = 0666
)

var (
	ErrClaimNotFound      = errors.New("claim not found")
	ErrClaimAlreadyExists = errors.New("claim already exists")
)

type Claim struct {
	UID     types.UID
	Address ghw.PCIAddress
}

type ClaimStore interface {
	Create(uid types.UID, addr ghw.PCIAddress) error
	Get(uid types.UID) (*ghw.PCIAddress, error)
	Delete(uid types.UID) (*ghw.PCIAddress, error)
	DeleteAll() error
	List() ([]Claim, error)
}

type fileClaimStore struct {
	rootDir    string
	isTAPStore bool
}

func NewFileClaimStore(rootDir string, isTAPStore bool) (ClaimStore, error) {
	if err := os.MkdirAll(rootDir, perm); err != nil {
		return nil, fmt.Errorf("error creating directory at %s: %w", rootDir, err)
	}
	return &fileClaimStore{rootDir, isTAPStore}, nil
}

func (s *fileClaimStore) claimFile(uid types.UID) string {
	return filepath.Join(s.rootDir, string(uid))
}

func (s *fileClaimStore) Create(uid types.UID, addr ghw.PCIAddress) error {
	filename := s.claimFile(uid)
	_, err := os.Stat(filename)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error stat-ing %s: %w", filename, err)
	}
	if err == nil {
		return ErrClaimAlreadyExists
	}

	data := []byte(addr.String())
	return os.WriteFile(filename, data, filePerm)
}

func (s *fileClaimStore) Get(uid types.UID) (*ghw.PCIAddress, error) {
	data, err := os.ReadFile(s.claimFile(uid))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("error reading pci address: %w", err)
		}
		return nil, ErrClaimNotFound
	}

	var addr *ghw.PCIAddress
	if !s.isTAPStore {
		addr = ghw.PCIAddressFromString(string(data))
	} else {
		addr = &ghw.PCIAddress{
			Device: string(data),
		}
	}
	if addr == nil {
		return nil, fmt.Errorf("invalid pci address %q", string(data))
	}

	return addr, nil
}

func (s *fileClaimStore) Delete(uid types.UID) (*ghw.PCIAddress, error) {
	addr, err := s.Get(uid)
	if err != nil {
		return nil, err
	}

	if err := os.Remove(s.claimFile(uid)); err != nil {
		return nil, fmt.Errorf("error deleting pci address: %w", err)
	}
	return addr, nil
}

func (s *fileClaimStore) DeleteAll() error {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return fmt.Errorf("error reading dir %s: %w", s.rootDir, err)
	}

	for _, entry := range entries {
		if err := os.Remove(filepath.Join(s.rootDir, entry.Name())); err != nil {
			return fmt.Errorf("error deleting claim file %s: %w", entry.Name(), err)
		}
	}

	return nil
}

func (s *fileClaimStore) List() ([]Claim, error) {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return nil, fmt.Errorf("error reading dir %s: %w", s.rootDir, err)
	}

	var claims []Claim
	for _, entry := range entries {
		uid := types.UID(entry.Name())
		addr, err := s.Get(uid)
		if err != nil {
			return nil, fmt.Errorf("[claim %s] error getting claim: %w", uid, err)
		}

		claims = append(claims, Claim{UID: uid, Address: *addr})
	}
	return claims, nil
}

var (
	ErrNoAddressAvailable = errors.New("no address available")
)

type Manager struct {
	store     ClaimStore
	available sets.Set[ghw.PCIAddress]
}

func NewManager(store ClaimStore, initAvailable []ghw.PCIAddress) (*Manager, error) {
	claims, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("error listing claims: %w", err)
	}

	available := sets.New(initAvailable...)
	for _, claim := range claims {
		if !available.Has(claim.Address) {
			return nil, fmt.Errorf("claim %s cannot claim non-existent address %s", claim.UID, &claim.Address)
		}

		available.Delete(claim.Address)
	}

	return &Manager{
		store:     store,
		available: available,
	}, nil
}

func (m *Manager) GetOrClaim(uid types.UID) (*ghw.PCIAddress, error) {
	addr, err := m.store.Get(uid)
	if err != nil && !errors.Is(err, ErrClaimNotFound) {
		return nil, fmt.Errorf("error getting claim: %w", err)
	}
	if err == nil {
		return addr, nil
	}

	newAddr, ok := m.available.PopAny()
	if !ok {
		return nil, ErrNoAddressAvailable
	}
	if err := m.store.Create(uid, newAddr); err != nil {
		m.available.Insert(newAddr)
		return nil, err
	}
	return &newAddr, nil
}

func (m *Manager) Get(uid types.UID) (*ghw.PCIAddress, error) {
	return m.store.Get(uid)
}

func (m *Manager) Release(uid types.UID) error {
	addr, err := m.store.Delete(uid)
	if err != nil {
		return err
	}

	m.available.Insert(*addr)
	return nil
}

func (m *Manager) ReleaseAll() error {
	if err := m.store.DeleteAll(); err != nil {
		return err
	}
	return nil
}

const (
	mellanoxVendorID         = "15b3"
	networkControllerClassID = "02"
)

func CollectVirtualFunctions(fs sysfs.FS) ([]ghw.PCIAddress, error) {
	pci, err := ghw.PCI()
	if err != nil {
		return nil, err
	}

	var addresses []ghw.PCIAddress
	for _, dev := range pci.Devices {
		if dev.Product.VendorID != mellanoxVendorID || dev.Class.ID != networkControllerClassID {
			continue
		}

		address := *ghw.PCIAddressFromString(dev.Address)
		sysFSDev, err := fs.PCIDevice(address)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("error looking up sysfs pci device %s: %w", &address, err)
			}
			continue
		}

		sriov, err := sysFSDev.SRIOV()
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("error getting sriov for %s: %w", dev.Address, err)
			}
			continue
		}
		if sriov.NumVFs == 0 {
			continue
		}

		virtFns, err := sysFSDev.Virtfns()
		if err != nil {
			return nil, fmt.Errorf("error discovering virtual functions for %s: %w", dev.Address, err)
		}

		for _, virtFn := range virtFns {
			virtFnAddr, err := virtFn.Address()
			if err != nil {
				return nil, fmt.Errorf("error getting virtual function %s address: %w", virtFn, err)
			}

			addresses = append(addresses, *virtFnAddr)
		}
	}
	return addresses, nil
}

func CollectTAPFunctions(devices []string) ([]ghw.PCIAddress, error) {
	var addresses []ghw.PCIAddress

	// Fill the addresses array
	for _, device := range devices {
		addr := ghw.PCIAddress{
			Device: device,
		}
		addresses = append(addresses, addr)
	}
	return addresses, nil
}

func GenerateVirtualFunctions(pfDevice string, numVFs int, offset int) ([]ghw.PCIAddress, error) {
	var addresses []ghw.PCIAddress

	baseAddress := *ghw.PCIAddressFromString(pfDevice)

	baseDevice, err := strconv.Atoi(baseAddress.Device)
	if err != nil {
		return nil, fmt.Errorf("error parsing device number: %v", err)
	}
	baseFunction, err := strconv.Atoi(baseAddress.Function)
	if err != nil {
		return nil, fmt.Errorf("error parsing function number: %v", err)
	}

	for i := 0; i < numVFs; i++ {
		device := baseDevice
		function := baseFunction + offset + i

		if function > 0x7 {
			device += function / 0x8
			function %= 0x8
		}

		newAddressStr := fmt.Sprintf("%s:%s:%02x.%x", baseAddress.Domain, baseAddress.Bus, device, function)
		newAddress := *ghw.PCIAddressFromString(newAddressStr)
		addresses = append(addresses, newAddress)
	}

	return addresses, nil
}
