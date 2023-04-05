package e2e

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/topolvm/topolvm"
	appsv1 "k8s.io/api/apps/v1"
)

func testSanity() {
	baseDir := "/tmp/topolvm/worker1/plugins/" + topolvm.GetPluginName() + "/"
	if isDaemonsetLvmdEnvSet() {
		baseDir = "/var/lib/kubelet/plugins/" + topolvm.GetPluginName() + "/"
	}

	It("should add node selector to node DaemonSet for CSI test", func() {
		// Skip deleting node because there is just one node in daemonset lvmd test environment.
		skipIfDaemonsetLvmd()

		if os.Getenv("SANITY_TEST_WITH_THIN_DEVICECLASS") == "true" {
			// Normally this node is deleted in testCleanup
			_, err := kubectl("delete", "nodes", "topolvm-e2e-worker3")
			Expect(err).ShouldNot(HaveOccurred())
		}
		_, err := kubectl("delete", "nodes", "topolvm-e2e-worker2")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			var ds appsv1.DaemonSet
			err := getObjects(&ds, "ds", "-n", "topolvm-system", "topolvm-node")
			if err != nil {
				return err
			}
			fmt.Println("topolvm-node", "ds.Status.NumberAvailable", ds.Status.NumberAvailable)
			if ds.Status.NumberAvailable != 1 {
				return errors.New("node daemonset is not ready")
			}
			return nil
		}).Should(Succeed())
	})

	tc := sanity.NewTestConfig()
	tc.Address = path.Join(baseDir, "/node/csi-topolvm.sock")
	tc.ControllerAddress = path.Join(baseDir, "/controller/csi-topolvm.sock")
	tc.TargetPath = path.Join(baseDir, "/node/mountdir")
	tc.StagingPath = path.Join(baseDir, "/node/stagingdir")
	tc.TestVolumeSize = 1073741824
	tc.IDGen = &sanity.DefaultIDGenerator{}
	tc.CheckPath = func(path string) (sanity.PathKind, error) {
		_, err := kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "-f", path)
		if err == nil {
			return sanity.PathIsFile, nil
		}
		_, err = kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "-d", path)
		if err == nil {
			return sanity.PathIsDir, nil
		}
		_, err = kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "!", "-e", path)
		if err == nil {
			return sanity.PathIsNotFound, nil
		}
		_, err = kubectl("exec", "-n", "topolvm-system", "daemonset/topolvm-node", "--", "test", "-e", path)
		return sanity.PathIsOther, err
	}

	if os.Getenv("SANITY_TEST_WITH_THIN_DEVICECLASS") == "true" {
		// csi.storage.k8s.io/fstype=xfs,topolvm.(io|cybozu.com)/device-class=thin
		volParams := make(map[string]string)
		volParams["csi.storage.k8s.io/fstype"] = "xfs"
		volParams[topolvm.GetDeviceClassKey()] = "thin"
		tc.TestVolumeParameters = volParams
	}
	sanity.GinkgoTest(&tc)
}
