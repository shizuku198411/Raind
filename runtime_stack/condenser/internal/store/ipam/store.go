package ipam

import (
	"condenser/internal/utils"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

func NewIpamStore(path string) *IpamStore {
	return &IpamStore{
		path:              path,
		filesystemHandler: utils.NewFilesystemExecutor(),
	}
}

type IpamStore struct {
	path              string
	mu                sync.Mutex
	filesystemHandler utils.FilesystemHandler
}

func (s *IpamStore) withLock(fn func(st *IpamState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.filesystemHandler.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()

	if err := s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_UN)

	st, err := s.loadOrInit()
	if err != nil {
		return err
	}

	if err := fn(st); err != nil {
		return err
	}

	return s.atomicSave(st)
}

func (s *IpamStore) withRLock(fn func(st *IpamState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.filesystemHandler.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()

	if err := s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_UN)

	st, err := s.loadOrInit()
	if err != nil {
		return err
	}

	if err := fn(st); err != nil {
		return err
	}

	return nil
}

func (s *IpamStore) loadOrInit() (*IpamState, error) {
	b, err := s.filesystemHandler.ReadFile(s.path)

	if err != nil {
		defaultHostInterface, getIfErr := GetDefaultInterfaceIpv4()
		if getIfErr != nil {
			return nil, getIfErr
		}
		defaultHostInterfaceAddr, getIfAddrErr := GetDefaultInterfaceAddressIpv4(defaultHostInterface)
		if getIfAddrErr != nil {
			return nil, getIfAddrErr
		}
		if s.filesystemHandler.IsNotExist(err) {
			// ipam state file not exist
			return &IpamState{
				Version:       "0.1.0",
				RuntimeSubnet: "10.166.0.0/16",
				DnsProxy: DnsProxy{
					DnsProxyInterface: "raindDns",
					DnsProxyAddr:      "10.166.254.254",
					Upstreams: []string{
						"8.8.8.8",
						"1.1.1.1",
					},
				},
				HostInterface:     defaultHostInterface,
				HostInterfaceAddr: defaultHostInterfaceAddr,
				Pools: []Pool{
					{
						Interface:   "raind0",
						Subnet:      "10.166.0.0/24",
						Address:     "10.166.0.254/24",
						Allocations: map[string]Allocation{},
					},
				},
			}, nil
		}
		return nil, err
	}

	var st IpamState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("ipam state json broken: %w", err)
	}
	for _, p := range st.Pools {
		if p.Allocations == nil {
			p.Allocations = map[string]Allocation{}
		}
	}
	return &st, nil
}

func (s *IpamStore) atomicSave(st *IpamState) error {
	tmp := s.path + ".tmp"

	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	f, err := s.filesystemHandler.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return s.filesystemHandler.Rename(tmp, s.path)
}

func (s *IpamStore) SetConfig() error {
	defaultHostInterface, err := GetDefaultInterfaceIpv4()
	if err != nil {
		return err
	}

	defaultHostInterfaceAddr, err := GetDefaultInterfaceAddressIpv4(defaultHostInterface)
	if err != nil {
		return err
	}

	return s.withLock(func(st *IpamState) error {
		st.Version = "0.1.0"
		st.RuntimeSubnet = "10.166.0.0/16"
		st.DnsProxy = DnsProxy{
			DnsProxyInterface: "raindDns",
			DnsProxyAddr:      "10.166.254.254",
			Upstreams: []string{
				"8.8.8.8",
				"1.1.1.1",
			},
		}
		st.HostInterface = defaultHostInterface
		st.HostInterfaceAddr = defaultHostInterfaceAddr
		if len(st.Pools) == 0 {
			st.Pools = append(st.Pools, Pool{
				Interface:   "raind0",
				Subnet:      "10.166.0.0/24",
				Address:     "10.166.0.254/24",
				Allocations: map[string]Allocation{},
			})
		}
		return nil
	})
}
