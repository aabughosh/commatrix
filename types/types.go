package types

import (
	"bytes"
	"cmp"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/openshift-kni/commatrix/consts"
	"sigs.k8s.io/yaml"
)

type Format int

const (
	FormatErr        = -1
	JSON      Format = iota
	YAML
	CSV
)

const (
	FormatJSON = "json"
	FormatYAML = "yaml"
	FormatCSV  = "csv"
)

type ComMatrix struct {
	Matrix []ComDetails
}

type ComDetails struct {
	Direction string `json:"direction" yaml:"direction"`
	Protocol  string `json:"protocol" yaml:"protocol"`
	Port      int    `json:"port" yaml:"port"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Service   string `json:"service" yaml:"service"`
	Pod       string `json:"pod" yaml:"pod"`
	Container string `json:"container" yaml:"container"`
	NodeRole  string `json:"nodeRole" yaml:"nodeRole"`
	Optional  bool   `json:"optional" yaml:"optional"`
}

func ToCSV(m ComMatrix) ([]byte, error) {
	out := make([]byte, 0)
	w := bytes.NewBuffer(out)
	csvwriter := csv.NewWriter(w)

	err := csvwriter.Write(strings.Split(consts.CSVHeaders, ","))
	if err != nil {
		return nil, fmt.Errorf("failed to write to CSV: %w", err)
	}

	for _, cd := range m.Matrix {
		record := strings.Split(cd.String(), ",")
		err := csvwriter.Write(record)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to CSV foramt: %w", err)
		}
	}
	csvwriter.Flush()

	return w.Bytes(), nil
}

func ToJSON(m ComMatrix) ([]byte, error) {
	out, err := json.MarshalIndent(m.Matrix, "", "    ")
	if err != nil {
		return nil, err
	}

	return out, nil
}

func ToYAML(m ComMatrix) ([]byte, error) {
	out, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (m *ComMatrix) String() string {
	var result strings.Builder
	for _, details := range m.Matrix {
		result.WriteString(details.String() + "\n")
	}

	return result.String()
}

func (cd ComDetails) String() string {
	return fmt.Sprintf("%s,%s,%d,%s,%s,%s,%s,%s,%v", cd.Direction, cd.Protocol, cd.Port, cd.Namespace, cd.Service, cd.Pod, cd.Container, cd.NodeRole, cd.Optional)
}

func CleanComDetails(outPuts []ComDetails) []ComDetails {
	allKeys := make(map[string]bool)
	res := []ComDetails{}
	for _, item := range outPuts {
		str := fmt.Sprintf("%s-%d-%s", item.NodeRole, item.Port, item.Protocol)
		if _, value := allKeys[str]; !value {
			allKeys[str] = true
			res = append(res, item)
		}
	}

	slices.SortFunc(res, func(a, b ComDetails) int {
		res := cmp.Compare(a.NodeRole, b.NodeRole)
		if res != 0 {
			return res
		}

		res = cmp.Compare(a.Protocol, b.Protocol)
		if res != 0 {
			return res
		}

		return cmp.Compare(a.Port, b.Port)
	})

	return res
}

func (cd ComDetails) Equals(other ComDetails) bool {
	strComDetail1 := fmt.Sprintf("%s-%d-%s", cd.NodeRole, cd.Port, cd.Protocol)
	strComDetail2 := fmt.Sprintf("%s-%d-%s", other.NodeRole, other.Port, other.Protocol)

	return strComDetail1 == strComDetail2
}

// Diff returns the diff ComMatrix.
func (m ComMatrix) Diff(other ComMatrix) ComMatrix {
	diff := []ComDetails{}
	for _, cd1 := range m.Matrix {
		found := false
		for _, cd2 := range other.Matrix {
			if cd1.Equals(cd2) {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, cd1)
		}
	}

	return ComMatrix{Matrix: diff}
}

func (m ComMatrix) Contains(cd ComDetails) bool {
	for _, cd1 := range m.Matrix {
		if cd1.Equals(cd) {
			return true
		}
	}

	return false
}
func ToNFTables(m ComMatrix, role string) []byte {
	var tcpPorts []string
	var udpPorts []string
	for _, line := range m.Matrix {
		if line.NodeRole != role {
			continue
		}
		if line.Protocol == "TCP" {
			tcpPorts = append(tcpPorts, fmt.Sprint(line.Port))
		} else if line.Protocol == "UDP" {
			udpPorts = append(udpPorts, fmt.Sprint(line.Port))
		}
	}

	tcpPortsStr := strings.Join(tcpPorts, ", ")
	udpPortsStr := strings.Join(udpPorts, ", ")

	result := fmt.Sprintf(`#!/usr/sbin/nft -f

	table ip filter {
		chain FIREWALL {
			# Allow loopback traffic
			type filter hook input priority 0; policy accept;
			iif lo accept
	
			# Allow established and related traffic
			ct state established,related accept
	
			# Allow SSH, DHCP, ICMP
			tcp dport { 22 } accept
			udp dport { 67, 68 } accept
			ip protocol icmp accept

			# Allow specific TCP and UDP ports
			tcp dport  { %s } accept
			udp dport { %s } accept

			# Logging and default drop
			log prefix "firewall " drop
		}
	
		chain INPUT {
			type filter hook input priority 0; policy accept;
			jump FIREWALL
		}
	}`, tcpPortsStr, udpPortsStr)

	return []byte(result)
}
