package systemd

import (
	"fmt"
	"os"

	vkmanager "github.com/virtual-kubelet/node-cli/manager"
	"github.com/virtual-kubelet/node-cli/provider"
	"github.com/virtual-kubelet/systemk/pkg/manager"
	"github.com/virtual-kubelet/systemk/pkg/packages"
	"github.com/virtual-kubelet/systemk/pkg/system"
	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// unitDir is where systemk stores the modified unit files.
var unitDir = "/var/run/systemk"

// P is a systemd provider.
type P struct {
	m   manager.Manager
	pkg packages.PackageManager
	rm  *vkmanager.ResourceManager
	w   *Watcher

	Addresses     []corev1.NodeAddress
	DaemonPort    int32
	ClusterDomain string
}

// New returns a new systemd provider.
func New(cfg provider.InitConfig) (*P, error) {
	if err := os.MkdirAll(unitDir, 0750); err != nil {
		return nil, err
	}
	m, err := manager.New(unitDir, false)
	if err != nil {
		return nil, err
	}
	p := &P{m: m}
	switch system.ID() {
	default:
		return nil, fmt.Errorf("unsupported system")
	case "debian", "ubuntu":
		p.pkg = new(packages.DebianPackageManager)

		// Just installed pre-requisites instead of pointing to the docs.
		klog.Infof("Installing %s, to prevent installed daemons from starting", "policyrcd-script-zg2")
		ok, err := p.pkg.Install("policyrcd-script-zg2", "")
		if err != nil {
			klog.Warningf("Failed to install %s, %s. Continuing anyway", "policyrcd-script-zg2", err)
		}
		if ok {
			klog.Infof("%s is already installed", "policyrcd-script-zg2")
		}

	case "arch":
		p.pkg = new(packages.ArchlinuxPackageManager)
	case "noop":
		p.pkg = new(packages.NoopPackageManager)
	}

	p.rm = cfg.ResourceManager
	p.DaemonPort = cfg.DaemonPort
	p.ClusterDomain = cfg.KubeClusterDomain

	if cfg.ConfigPath == "" {
		return p, nil
	}

	clientset, err := nodeutil.ClientsetFromEnv(cfg.ConfigPath)
	if err != nil {
		return p, err
	}
	// Get new clientset.
	w := newWatcher(clientset)
	go func() {
		if err := w.run(p); err != nil {
			klog.Fatal(err)
		}
	}()
	p.w = w
	return p, nil
}

var _ provider.Provider = new(P)
