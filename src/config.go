package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type dockerInfo struct {
	Bridge      string `yaml:"bridge"`
	Ipv4Subnet  string `yaml:"ipv4_subnet"`
	Ipv4Gateway string `yaml:"ipv4_gateway"`
	Ipv6Subnet  string `yaml:"ipv6_subnet"`
	Ipv6Gateway string `yaml:"ipv6_gateway"`
}

type dutInfo struct {
	Kind   string `yaml:"kind"`
	Group  string `yaml:"group"`
	Type   string `yaml:"type"`
	Config string `yaml:"config"`
}

type conf struct {
	Prefix      string             `yaml:"Prefix"`
	DockerInfo  dockerInfo         `yaml:"Docker_info"`
	ClientImage string             `yaml:"Client_image"`
	SRLImage    string             `yaml:"SRL_image"`
	SRLConfig   string             `yaml:"SRL_config"`
	SRLLicense  string             `yaml:"SRL_license"`
	CEOSImage   string             `yaml:"CEOS_image"`
	CEOSConfig  string             `yaml:"CEOS_config"`
	Duts        map[string]dutInfo `yaml:"Duts"`
	Links       []struct {
		Endpoints []string `yaml:"endpoints"`
	} `yaml:"Links"`
	ConfigPath string
}

type volume struct {
	Source      string
	Destination string
	ReadOnly    bool
}

// Node is a struct that contains the information of a container element
type Node struct {
	Name       string
	Index      int
	Group      string
	OS         string
	Config     string
	NodeType   string
	License    string
	Image      string
	Topology   string
	Path       string
	EnvConf    string
	TLS        string
	Checkpoint string
	Sysctls    map[string]string
	User       string
	Cmd        string
	Env        []string
	Mounts     map[string]volume
	Volumes    map[string]struct{}
	Binds      []string
	Pid        int
	Cid        string
	MgmtIPv4   string
	MgmtIPv6   string
	MgmtMac    string
}

// Link is a struct that contains the information of a link between 2 containers
type Link struct {
	a *Endpoint
	b *Endpoint
}

// Endpoint is a sttruct that contains information of a link endpoint
type Endpoint struct {
	Node *Node
	// e1-x, eth, etc
	EndpointName string
}

func (c *cLab) parseIPInfo() error {
	//DockerInfo = t.DockerInfo
	if c.Conf.DockerInfo.Bridge == "" {
		c.Conf.DockerInfo.Bridge = "srlinux_bridge"
	}
	if c.Conf.DockerInfo.Ipv4Subnet == "" {
		c.Conf.DockerInfo.Bridge = "172.19.19.0/24"
	}
	if c.Conf.DockerInfo.Ipv6Subnet == "" {
		c.Conf.DockerInfo.Bridge = "2001:172:19:19::/80"
	}

	_, ipv4Net, err := net.ParseCIDR(c.Conf.DockerInfo.Ipv4Subnet)
	if err != nil {
		return err
	}
	ipv4Gateway := ipv4Net.IP.To4()
	ipv4Gateway[3]++
	c.Conf.DockerInfo.Ipv4Gateway = ipv4Gateway.String()

	_, ipv6Net, err := net.ParseCIDR(c.Conf.DockerInfo.Ipv6Subnet)
	if err != nil {
		return err
	}
	ipv6Gateway := ipv6Net.IP
	ipv6Gateway[15]++
	c.Conf.DockerInfo.Ipv6Gateway = ipv6Gateway.String()

	return nil
}

func (c *cLab) parseTopology() error {
	// initialize Prefix
	//Prefix = t.Prefix
	log.Debug(fmt.Sprintf("Prefix: %s", c.Conf.Prefix))
	// initialize DockerInfo
	err := c.parseIPInfo()
	if err != nil {
		return err
	}
	log.Debug(fmt.Sprintf("DockerInfo: %v", c.Conf.DockerInfo))

	if c.Conf.ConfigPath == "" {
		c.Conf.ConfigPath, _ = filepath.Abs(os.Getenv("PWD"))
	}


	// initialize the Node information from the topology map
	idx := 0
	for dut, data := range c.Conf.Duts {
		c.Nodes[dut] = c.NewNode(dut, data, idx)
		idx++
	}
	for i, l := range c.Conf.Links {
		// i represnts the endpoint integer and l provide the lik struct
		c.Links[i] = c.NewLink(l.Endpoints)
	}
	return nil
}

// NewNode initializes a new node object
func (c *cLab) NewNode(dutName string, dut dutInfo, idx int) *Node {
	// initialize a new node
	node := new(Node)
	node.Name = dutName
	node.Index = idx
	// normalize the data to lower case to compare
	node.OS = strings.ToLower(dut.Kind)
	switch node.OS {
	case "ceos":
		// initialize the global parameters with defaults, can be overwritten later
		node.Config = c.Conf.CEOSConfig
		//node.License = t.SRLLicense
		node.Image = c.Conf.CEOSImage
		//node.NodeType = "ixr6"

		// initialize specifc container information
		node.Cmd = "/sbin/init systemd.setenv=INTFTYPE=eth systemd.setenv=ETBA=1 systemd.setenv=SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT=1 systemd.setenv=CEOS=1 systemd.setenv=EOS_PLATFORM=ceoslab systemd.setenv=container=docker"
		//node.Cmd = "/sbin/init"
		node.Env = []string{
			"CEOS=1",
			"EOS_PLATFORM=ceoslab",
			"container=docker",
			"ETBA=1",
			"SKIP_ZEROTOUCH_BARRIER_IN_SYSDBINIT=1",
			"INTFTYPE=eth"}
		node.User = "root"
		node.Group = dut.Group
		node.NodeType = dut.Type
		node.Config = dut.Config

		node.Sysctls = make(map[string]string)
		node.Sysctls["net.ipv4.ip_forward"] = "0"
		node.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
		node.Sysctls["net.ipv6.conf.all.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.default.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.all.autoconf"] = "0"
		node.Sysctls["net.ipv6.conf.default.autoconf"] = "0"

	case "srl":
		// initialize the global parameters with defaults, can be overwritten later
		node.Config = c.Conf.SRLConfig
		node.License = c.Conf.SRLLicense
		node.Image = c.Conf.SRLImage
		node.NodeType = "ixr6"

		// initialize specifc container information
		node.Cmd = "sudo bash -c /opt/srlinux/bin/sr_linux"
		node.Env = []string{"SRLINUX=1"}
		node.User = "root"

		node.Group = dut.Group
		node.NodeType = dut.Type
		node.Config = dut.Config

		switch node.NodeType {
		case "ixr6":
			node.Topology = "srl_config/templates/topology-7250IXR6.yml"
		case "ixr10":
			node.Topology = "srl_config/templates/topology-7250IXR10.yml"
		case "ixrd1":
			node.Topology = "srl_config/templates/topology-7220IXD1.yml"
		case "isrd2":
			node.Topology = "srl_config/templates/topology-7220IXD2.yml"
		case "ixrd3":
			node.Topology = "srl_config/templates/topology-7220IXD3.yml"
		default:
			panic("wrong node type; should be ixr6, ixr10, ixrd1, ixrd2, ixrd3")
		}

		node.Sysctls = make(map[string]string)
		node.Sysctls["net.ipv4.ip_forward"] = "0"
		node.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
		node.Sysctls["net.ipv6.conf.all.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.default.accept_dad"] = "0"
		node.Sysctls["net.ipv6.conf.all.autoconf"] = "0"
		node.Sysctls["net.ipv6.conf.default.autoconf"] = "0"

		node.Mounts = make(map[string]volume)
		var v volume
		labPath := c.Conf.ConfigPath + "/" + "lab" + "-" + c.Conf.Prefix + "/"
		labDutPath := labPath + dutName + "/"
		v.Source = labPath + "license.key"
		v.Destination = "/opt/srlinux/etc/license.key"
		v.ReadOnly = true
		log.Debug("License key: ", v.Source)
		node.Mounts["license"] = v

		v.Source = labDutPath + "config/"
		v.Destination = "/etc/opt/srlinux/"
		v.ReadOnly = false
		log.Debug("Config: ", v.Source)
		node.Mounts["config"] = v

		v.Source = labDutPath + "srlinux.conf"
		v.Destination = "/home/admin/.srlinux.conf"
		v.ReadOnly = false
		log.Debug("Env Config: ", v.Source)
		node.Mounts["envConf"] = v

		// v.Source = labDutPath + "tls/"
		// v.Destination = "/etc/opt/srlinux/tls/"
		// v.ReadOnly = false
		// log.Debug("TLS Dir: ", v.Source)
		// node.Mounts["tls"] = v

		// v.Source = labDutPath + "checkpoint/"
		// v.Destination = "/etc/opt/srlinux/checkpoint/"
		// v.ReadOnly = false
		// log.Debug("checkPoint Dir: ", v.Source)
		// node.Mounts["checkPoint"] = v

		v.Source = labDutPath + "topology.yml"
		v.Destination = "/tmp/topology.yml"
		v.ReadOnly = true
		log.Debug("Topology File: ", v.Source)
		node.Mounts["topology"] = v

		node.Volumes = make(map[string]struct{})
		node.Volumes = map[string]struct{}{
			node.Mounts["license"].Destination:  struct{}{},
			node.Mounts["config"].Destination:   struct{}{},
			node.Mounts["envConf"].Destination:  struct{}{},
			node.Mounts["topology"].Destination: struct{}{},
		}

		bindLicense := node.Mounts["license"].Source + ":" + node.Mounts["license"].Destination + ":" + "ro"
		bindConfig := node.Mounts["config"].Source + ":" + node.Mounts["config"].Destination + ":" + "rw"
		bindEnvConf := node.Mounts["envConf"].Source + ":" + node.Mounts["envConf"].Destination + ":" + "rw"
		bindTopology := node.Mounts["topology"].Source + ":" + node.Mounts["topology"].Destination + ":" + "ro"

		node.Binds = append(node.Binds, bindLicense)
		node.Binds = append(node.Binds, bindConfig)
		node.Binds = append(node.Binds, bindEnvConf)
		node.Binds = append(node.Binds, bindTopology)

	case "alpine", "linux":
		node.Image = c.Conf.ClientImage
		node.Cmd = "/bin/bash"

		node.Group = dut.Group
		node.NodeType = dut.Type
		node.Config = dut.Config

	}
	return node
}

// NewLink initializes a new link object
func (c *cLab) NewLink(e []string) *Link {
	// initialize a new link
	link := new(Link)

	for i, d := range e {
		// i indicates the number and d presents the string, which need to be
		// split in node and endpoint name
		if i == 0 {
			link.a = c.NewEndpoint(d)
		} else {
			link.b = c.NewEndpoint(d)
		}
	}
	return link
}

// NewEndpoint initializes a new endpoint object
func (c *cLab) NewEndpoint(e string) *Endpoint {
	// initialize a new endpoint
	endpoint := new(Endpoint)

	// split the string to get node name and endpoint name
	split := strings.Split(e, ":")
	// search the node pointer based in the name of the split function
	found := false
	for name, n := range c.Nodes {
		if name == split[0] {
			endpoint.Node = n
			found = true
		} else {

		}
	}
	if !found {
		log.Error("Not all nodes are specified in the duts section or the names dont match in the duts/endpoint section", split[0])
		os.Exit(1)
	}

	// initialize the endpoint name based on the split function
	endpoint.EndpointName = split[1]

	return endpoint
}
