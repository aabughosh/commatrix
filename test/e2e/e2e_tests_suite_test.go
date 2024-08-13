package e2e

import (
	"context"
	"log"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sync/errgroup"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift-kni/commatrix/pkg/client"
	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	"github.com/openshift-kni/commatrix/pkg/types"
	"github.com/openshift-kni/commatrix/pkg/utils"
	"github.com/openshift-kni/commatrix/test/pkg/firewall"
	nodeWrapper "github.com/openshift-kni/commatrix/test/pkg/node"
)

var (
	cs           *client.ClientSet
	matrix       *types.ComMatrix
	isSNO        bool
	utilsHelpers utils.UtilsInterface
)

var _ = BeforeSuite(func() {
	By("generating the commatrix")
	var err error
	cs, err = client.New()
	Expect(err).NotTo(HaveOccurred())

	deployment := types.Standard
	isSNO, err := isSNOCluster(cs)
	Expect(err).NotTo(HaveOccurred())

	if isSNO {
		deployment = types.SNO
	}

	infra := types.Cloud
	isBM, err := isBMInfra(cs)
	Expect(err).NotTo(HaveOccurred())

	if isBM {
		infra = types.Baremetal
	}

	epExporter, err := endpointslices.New(cs)
	Expect(err).ToNot(HaveOccurred())

	commMatrix, err := commatrixcreator.New(epExporter, "", "", infra, deployment)
	Expect(err).NotTo(HaveOccurred())

	matrix, err = commMatrix.CreateEndpointMatrix()
	Expect(err).NotTo(HaveOccurred())
	utilsHelpers = utils.New(cs)
})

var _ = Describe("commatrix", func() {
	It("should apply firewall by blocking all ports except the ones OCP is listening on", func() {
		By("apply firewall on nodes")
		masterMat, workerMat := matrix.SeparateMatrixByRole()
		var workerNFT []byte

		masterNFT, err := masterMat.ToNFTables()
		Expect(err).NotTo(HaveOccurred())
		if !isSNO {
			workerNFT, err = workerMat.ToNFTables()
			Expect(err).NotTo(HaveOccurred())
		}
		ns := "commatrix-firewall"
		err = utilsHelpers.CreateNamespace(ns)
		Expect(err).ToNot(HaveOccurred())

		defer func() {
			err := utilsHelpers.DeleteNamespace(ns)
			Expect(err).ToNot(HaveOccurred())

		}()

		nodeList := &corev1.NodeList{}
		err = cs.List(context.TODO(), nodeList)
		Expect(err).ToNot(HaveOccurred())

		firewallapply := firewall.New(ns, utilsHelpers)

		g := new(errgroup.Group)

		for _, node := range nodeList.Items {
			nodeName := node.Name
			nodeRole, err := types.GetNodeRole(&node)
			Expect(err).ToNot(HaveOccurred())
			g.Go(func() error {
				var nftTable []byte
				if nodeRole == "master" {
					nftTable = masterNFT
				} else {
					nftTable = workerNFT
				}

				err := firewallapply.ApplyRulesToNode(nftTable, nodeName)
				if err != nil {
					return err
				}
				return nil
			})

		}
		// Wait for all goroutines to finish
		err = g.Wait()
		Expect(err).ToNot(HaveOccurred())

		By("reboot first node")
		Expect(err).ToNot(HaveOccurred())

		nodeWrapper := nodeWrapper.New(&nodeList.Items[0], cs)
		err = nodeWrapper.SoftRebootNodeAndWaitForDisconnect(ns)
		Expect(err).ToNot(HaveOccurred())

		nodeWrapper.WaitForNodeReady()

		firewallcheck := firewall.New(ns, utilsHelpers)

		output, err := firewallcheck.RulesList(nodeList.Items[0].Name)
		Expect(err).ToNot(HaveOccurred())

		if strings.Contains(string(output), "table inet openshift_filter") &&
			strings.Contains(string(output), "chain OPENSHIFT") {
			log.Println("The byte slices are identical.")
		} else {
			Fail("The byte slices are different")
		}
	})
})

func isSNOCluster(cs *client.ClientSet) (bool, error) {
	oc := configv1client.NewForConfigOrDie(cs.Config)
	infra, err := oc.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	return infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode, nil
}

func isBMInfra(cs *client.ClientSet) (bool, error) {
	oc := configv1client.NewForConfigOrDie(cs.Config)
	infra, err := oc.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	return infra.Status.PlatformStatus.Type == configv1.BareMetalPlatformType, nil
}