package generate

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/openshift-kni/commatrix/pkg/client"
	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	"github.com/openshift-kni/commatrix/pkg/consts"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"
	listeningsockets "github.com/openshift-kni/commatrix/pkg/listening-sockets"
	matrixdiff "github.com/openshift-kni/commatrix/pkg/matrix-diff"
	"github.com/openshift-kni/commatrix/pkg/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	k8sapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift-kni/commatrix/pkg/types"
)

var (
	commatrixLong = templates.LongDesc(`
			  Generate an up-to-date communication flows matrix for all ingress flows of Kubernetes (multi-node and single-node in OpenShift) and Operators.

			  This command will generate the communication flows matrix, the SS matrix, and the difference between them.

			  For additional details, please refer to the communication matrix documentation(https://github.com/openshift-kni/commatrix/blob/main/README.md)

	`)
	CommatrixExample = templates.Examples(`
			 # Generate all communication matrices including SS, EndpointSlice, and their differences in selected format:
			 kubectl commatrix generate --host-open-ports --format (json|yaml|csv|nft)
			
			 # Generate the endpointSlice matrix in json format with custom entries:
			 kubectl commatrix generate --format json --customEntriesPath /path/to/customEntriesFile --customEntriesFormat json
			
			 # Generate the matrix in json format with debug logs:
			 kubectl commatrix generate --format json --debug
	`)
)

var (
	validFormats = []string{
		types.FormatCSV,
		types.FormatJSON,
		types.FormatYAML,
		types.FormatNFT,
	}

	validCustomEntriesFormats = []string{
		types.FormatCSV,
		types.FormatJSON,
		types.FormatYAML,
	}
)

type GenerateOptions struct {
	destDir             string
	format              string
	customEntriesPath   string
	customEntriesFormat string
	debug               bool
	openPorts           bool
	cs                  *client.ClientSet
	utilsHelpers        utils.UtilsInterface
	configFlags         *genericclioptions.ConfigFlags
	genericiooptions.IOStreams
}

func NewCmd(cs *client.ClientSet, streams genericiooptions.IOStreams) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   "commatrix",
		Short: "Run communication matrix commands.",
		Long:  commatrixLong,
	}
	cmds.AddCommand(NewCmdCommatrixGenerate(cs, streams))

	return cmds
}

func NewCommatrixOptions(streams genericiooptions.IOStreams, cs *client.ClientSet) *GenerateOptions {
	return &GenerateOptions{
		configFlags:  genericclioptions.NewConfigFlags(true),
		IOStreams:    streams,
		cs:           cs,
		utilsHelpers: utils.New(cs),
	}
}

// NewCmdAddRoleToUser implements the OpenShift cli add-role-to-user command.
func NewCmdCommatrixGenerate(cs *client.ClientSet, streams genericiooptions.IOStreams) *cobra.Command {
	o := NewCommatrixOptions(streams, cs)
	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate an up-to-date communication flows matrix for all ingress flows.",
		Long:    commatrixLong,
		Example: CommatrixExample,
		RunE: func(c *cobra.Command, args []string) (err error) {
			if err := Validate(o); err != nil {
				return err
			}

			if err := Run(o); err != nil {
				return fmt.Errorf("failed to generate matrix: %v", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&o.destDir, "destDir", "communication-matrix", "Output files dir")
	cmd.Flags().StringVar(&o.format, "format", "csv", "Desired format (json,yaml,csv,nft)")
	cmd.Flags().BoolVar(&o.debug, "debug", false, "Debug logs")
	cmd.Flags().StringVar(&o.customEntriesPath, "customEntriesPath", "", "Add custom entries from a file to the matrix")
	cmd.Flags().StringVar(&o.customEntriesFormat, "customEntriesFormat", "", "Set the format of the custom entries file (json,yaml,csv)")
	cmd.Flags().BoolVar(&o.openPorts, "host-open-ports", false, "Generate communication matrices: EndpointSlice matrix, SS matrix, and their difference.")

	return cmd
}

func Validate(o *GenerateOptions) error {
	if !slices.Contains(validFormats, o.format) {
		return fmt.Errorf("invalid format '%s', valid options are: %s",
			o.format, strings.Join(validFormats, ", "))
	}

	if o.customEntriesPath == "" {
		return nil
	}

	if o.customEntriesFormat == "" {
		return fmt.Errorf("you must specify the --customEntriesFormat when using --customEntriesPath")
	}

	if !slices.Contains(validCustomEntriesFormats, o.customEntriesFormat) {
		return fmt.Errorf("invalid custom entries format '%s', valid options are: %s",
			o.customEntriesFormat, strings.Join(validCustomEntriesFormats, ", "))
	}

	return nil
}

func Run(o *GenerateOptions) (err error) {
	if o.debug {
		log.SetLevel(log.DebugLevel)
	}

	matrix, err := generateMatrix(o)
	if err != nil {
		return fmt.Errorf("failed to generate endpoint slice matrix: %v", err)
	}

	if o.openPorts {
		ssMat, err := generateSS(o)
		if err != nil {
			return fmt.Errorf("failed to generate SS matrix: %v", err)
		}

		log.Debug("Generating diff between the endpoint slice and SS matrix")
		diff := matrixdiff.Generate(matrix, ssMat)
		diffStr, err := diff.String()
		if err != nil {
			log.Panicf("Error while generating matrix diff string: %v", err)
		}

		log.Debug("Writing the matrix diff to file")
		err = o.utilsHelpers.WriteFile(filepath.Join(o.destDir, "matrix-diff-ss"), []byte(diffStr))
		if err != nil {
			log.Panicf("Error writing the diff matrix file: %v", err)
		}

		log.Debug("Matrix diff successfully written to file")
	}
	return nil
}

func getInfraAndDeployemntType(cs *client.ClientSet, utilsHelpers utils.UtilsInterface) (types.Deployment, types.Env, error) {
	deployment := types.Standard
	infra := types.Cloud
	isOCP := isOpenShift(cs)

	if isOCP { // checking if it openshift cluster
		isSNO, err := utilsHelpers.IsSNOCluster()
		if err != nil {
			return deployment, infra, fmt.Errorf("failed to check is sno cluster %s", err)
		}

		if isSNO {
			deployment = types.SNO
		}

		isBM, err := utilsHelpers.IsBMInfra()
		if err != nil {
			return deployment, infra, fmt.Errorf("failed to check is bm cluster %s", err)
		}

		if isBM {
			infra = types.Baremetal
		}
	} else { // if not openshift cluster
		isBM, err := isK8SBMInfra(cs)
		if err != nil {
			return deployment, infra, fmt.Errorf("failed to check is bm cluster %s", err)
		}

		if isBM {
			infra = types.Baremetal
		}
	}
	return deployment, infra, nil
}

func generateMatrix(o *GenerateOptions) (*types.ComMatrix, error) {
	if o.debug {
		log.SetLevel(log.DebugLevel)
	}

	log.Debug("Detecting deployment and infra types")
	deployment, infra, err := getInfraAndDeployemntType(o.cs, o.utilsHelpers)
	if err != nil {
		return nil, fmt.Errorf("failed to determine Infra Deployemnt type: %v", err)
	}

	epExporter, err := endpointslices.New(o.cs)
	if err != nil {
		return nil, fmt.Errorf("failed creating the endpointslices exporter %s", err)
	}

	log.Debug("Creating communication matrix")
	commMatrix, err := commatrixcreator.New(epExporter, o.customEntriesPath, o.customEntriesFormat, infra, deployment)
	if err != nil {
		return nil, err
	}

	matrix, err := commMatrix.CreateEndpointMatrix()
	if err != nil {
		return nil, err
	}

	log.Debug("Writing endpoint matrix to file")
	err = matrix.WriteMatrixToFileByType(o.utilsHelpers, "communication-matrix", o.format, deployment, o.destDir)
	if err != nil {
		return nil, fmt.Errorf("failed to write endpoint matrix to file: %v", err)
	}

	return matrix, nil
}

func generateSS(o *GenerateOptions) (*types.ComMatrix, error) {
	if o.debug {
		log.SetLevel(log.DebugLevel)
	}

	log.Debug("Detecting deployment and infra types")
	deployment, _, err := getInfraAndDeployemntType(o.cs, o.utilsHelpers)
	if err != nil {
		return nil, fmt.Errorf("failed to determine Infra Deployemnt type: %v", err)
	}

	log.Debug("Creating listening socket check")
	listeningCheck, err := listeningsockets.NewCheck(o.cs, o.utilsHelpers, o.destDir)
	if err != nil {
		return nil, fmt.Errorf("failed creating listening socket check: %v", err)
	}

	log.Debug("Creating namespace")
	err = o.utilsHelpers.CreateNamespace(consts.DefaultDebugNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create namespace: %v", err)
	}

	log.Debug("Generating SS matrix and raw files")
	ssMat, ssOutTCP, ssOutUDP, err := listeningCheck.GenerateSS(consts.DefaultDebugNamespace)
	if err != nil {
		return nil, fmt.Errorf("error while generating the listening check matrix and ss raws: %v", err)
	}

	log.Debug("Writing SS raw files")
	err = listeningCheck.WriteSSRawFiles(ssOutTCP, ssOutUDP)
	if err != nil {
		return nil, fmt.Errorf("error while writing the SS raw files: %v", err)
	}

	log.Debug("Writing SS matrix to file")
	err = ssMat.WriteMatrixToFileByType(o.utilsHelpers, "ss-generated-matrix", o.format, deployment, o.destDir)
	if err != nil {
		return nil, fmt.Errorf("error while writing SS matrix to file: %v", err)
	}

	return ssMat, nil
}

func isOpenShift(cluster *client.ClientSet) bool {
	ctx := context.Background()
	_, err := cluster.Namespaces().Get(ctx, "openshift-kube-apiserver", metav1.GetOptions{})
	return !k8sapierror.IsNotFound(err)
}

func isK8SBMInfra(cs *client.ClientSet) (bool, error) {
	var nodeList corev1.NodeList
	err := cs.List(context.TODO(), &nodeList)
	if err != nil {
		return false, err
	}

	for _, node := range nodeList.Items {
		labels := node.Labels

		if _, exists := labels["topology.kubernetes.io/region"]; exists {
			return false, nil
		}
		if _, exists := labels["topology.kubernetes.io/zone"]; exists {
			return false, nil
		}
		if _, exists := labels["failure-domain.beta.kubernetes.io/region"]; exists {
			return false, nil
		}
		if _, exists := labels["failure-domain.beta.kubernetes.io/zone"]; exists {
			return false, nil
		}
	}

	return true, nil
}
