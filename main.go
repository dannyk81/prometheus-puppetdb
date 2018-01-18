package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"

	"gopkg.in/yaml.v2"
)

var version = "undefined"
var transport *http.Transport

type Config struct {
	Version         bool   `short:"V" long:"version" description:"Display version."`
	PuppetDBURL     string `short:"u" long:"puppetdb-url" description:"PuppetDB base URL." env:"PROMETHEUS_PUPPETDB_URL" default:"http://puppetdb:8080"`
	CertFile        string `short:"x" long:"cert-file" description:"A PEM encoded certificate file." env:"PROMETHEUS_CERT_FILE" default:"certs/client.pem"`
	KeyFile         string `short:"y" long:"key-file" description:"A PEM encoded private key file." env:"PROMETHEUS_KEY_FILE" default:"certs/client.key"`
	CACertFile      string `short:"z" long:"cacert-file" description:"A PEM encoded CA's certificate file." env:"PROMETHEUS_CACERT_FILE" default:"certs/cacert.pem"`
	SSLSkipVerify   bool   `short:"k" long:"ssl-skip-verify" description:"Skip SSL verification." env:"PROMETHEUS_SSL_SKIP_VERIFY"`
	Query           string `short:"q" long:"puppetdb-query" description:"PuppetDB query." env:"PROMETHEUS_PUPPETDB_QUERY" default:"facts[certname, value]"`
	Filter          string `short:"f" long:"puppetdb-filter" description:"PuppetDB filter." env:"PROMETHEUS_PUPPETDB_FILTER" default:"name='ipaddress' and nodes { deactivated is null }"`
	RoleMappingFile string `short:"r" long:"role-mapping-file" description:"Role mapping configuration file" env:"PROMETHEUS_ROLE_MAPPING_FILE" default:"role-mapping.yaml"`
	TargetsDir      string `short:"c" long:"targets-dir" description:"Directory to store File SD targets files." env:"PROMETHEUS_TARGETS_DIR" default:"/etc/prometheus/targets"`
	Sleep           string `short:"s" long:"sleep" description:"Sleep time between queries." env:"PROMETHEUS_PUPPETDB_SLEEP" default:"60s"`
	Manpage         bool   `short:"m" long:"manpage" description:"Output manpage."`
}

type Node struct {
	Certname  string `json:"certname"`
	Ipaddress string `json:"value"`
}

type RoleMapping struct {
	Exporter string   `yaml:"exporter"`
	Port     int      `yaml:"port"`
	Path     string   `yaml:"path"`
	Scheme   string   `yaml:"scheme"`
	Roles    []string `yaml:"roles"`
}

type Targets struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

func main() {
	cfg, err := loadConfig(version)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	puppetdbURL, err := url.Parse(cfg.PuppetDBURL)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if puppetdbURL.Scheme != "http" && puppetdbURL.Scheme != "https" {
		fmt.Printf("%s is not a valid http scheme\n", puppetdbURL.Scheme)
		os.Exit(1)
	}

	if puppetdbURL.Scheme == "https" {
		// Load client cert
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Load CA cert
		caCert, err := ioutil.ReadFile(cfg.CACertFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		// Setup HTTPS client
		tlsConfig := &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: cfg.SSLSkipVerify,
		}
		tlsConfig.BuildNameToCertificate()
		transport = &http.Transport{TLSClientConfig: tlsConfig}
	} else {
		transport = &http.Transport{}
	}

	// Setup the http client
	client := &http.Client{Transport: transport}

	// Start the main loop
	for {
		// Read the role mapping from configuration file
		roleMapping, err := loadRoleMapping(cfg.RoleMappingFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Clean the targets directory, remove any target files that are no longer listed in Role Mapping
		err = cleanupTargetsDir(cfg.TargetsDir, roleMapping)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Iterate through the Exporters
		for e := range roleMapping {
			var nodes []Node
			// Iterate through the Roles mapped to each Exporter
			for r := range roleMapping[e].Roles {
				var tmpNodes []Node
				// Get the nodes for this role
				tmpNodes, err = getNodes(client, cfg.PuppetDBURL, cfg.Query, cfg.Filter, roleMapping[e].Roles[r])
				if err != nil {
					fmt.Println(err)
					break
				}
				nodes = append(nodes, tmpNodes...)
			}

			// Write the nodes to a Targets file per Exporter (==Job)
			err = writeNodes(nodes, roleMapping[e].Port, roleMapping[e].Path, roleMapping[e].Scheme, roleMapping[e].Exporter, cfg.TargetsDir)
			if err != nil {
				fmt.Println(err)
				break
			}
		}

		// Sleep...
		sleep, err := time.ParseDuration(cfg.Sleep)
		if err != nil {
			fmt.Println(err)
			break
		}
		fmt.Printf("Sleeping for %v\n", sleep)
		time.Sleep(sleep)
	}
}

func loadConfig(version string) (c Config, err error) {
	parser := flags.NewParser(&c, flags.Default)
	_, err = parser.Parse()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if c.Version {
		fmt.Printf("Prometheus-puppetdb v%v\n", version)
		os.Exit(0)
	}

	if c.Manpage {
		var buf bytes.Buffer
		parser.ShortDescription = "Prometheus scrape lists based on PuppetDB"
		parser.WriteManPage(&buf)
		fmt.Printf(buf.String())
		os.Exit(0)
	}
	return
}

func loadRoleMapping(mappingFile string) (roleMapping []RoleMapping, err error) {
	filename, _ := filepath.Abs(mappingFile)
	yamlFile, err := ioutil.ReadFile(filename)

	err = yaml.Unmarshal(yamlFile, &roleMapping)
	if err != nil {
		return
	}

	return
}

// Iterate through the yml & yaml files in TargetsDir and remove all that do not match an Exporter in roleMapping
func cleanupTargetsDir(dir string, roles []RoleMapping) (err error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Println(err)
		return
	}

OUTER:
	for _, file := range files {
		for r := range roles {
			found, _ := regexp.MatchString(fmt.Sprintf("%s.(yaml|yml)", roles[r].Exporter), file.Name())
			if found {
				continue OUTER
			}
		}

		err = os.Remove(fmt.Sprintf("%s/%s", dir, file.Name()))
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	return
}

func getNodes(client *http.Client, puppetdb string, query string, filter string, role string) (nodes []Node, err error) {
	// This was temporary hack
	//q := fmt.Sprintf("facts[certname,value] {name='ipaddress' and nodes { deactivated is null } and facts { name='role' and value='%s' } }", role)

	// Build the query from Query, Filter and the role
	q := fmt.Sprintf("%s {%s and facts { name='role' and value='%s' } }", query, filter, role)

	form := strings.NewReader(fmt.Sprintf("{\"query\":\"%s\"}", q))
	puppetdbURL := fmt.Sprintf("%s/pdb/query/v4", puppetdb)
	req, err := http.NewRequest("POST", puppetdbURL, form)
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(body, &nodes)

	return
}

func writeNodes(nodes []Node, port int, path string, scheme string, job string, dir string) (err error) {
	allTargets := []Targets{}

	for _, node := range nodes {
		targets := Targets{}

		target := fmt.Sprintf("%s:%v", node.Ipaddress, port)
		targets.Targets = append(targets.Targets, target)
		targets.Labels = map[string]string{
			"job":          job,
			"certname":     node.Certname,
			"metrics_path": path,
			"scheme":       scheme,
		}
		allTargets = append(allTargets, targets)
	}

	d, err := yaml.Marshal(&allTargets)
	if err != nil {
		return
	}

	os.MkdirAll(fmt.Sprintf("%s", dir), 0755)
	err = ioutil.WriteFile(fmt.Sprintf("%s/%s.yml", dir, job), d, 0644)
	if err != nil {
		return
	}

	return nil
}
