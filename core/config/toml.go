package config

import (
	"bytes"
	"io/ioutil"
	"text/template"
	"time"
)

const defaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

home-dir = "{{ .HomeDir }}"
use-on-memory = {{ .UseOnMemory }}
shortcut-preparams = {{ .ShortcutPreparams }}
sisu-server-url = "{{ .SisuServerUrl }}"
port = {{ .Port }}

###############################################################################
###                        Database Configuration                           ###
###############################################################################
[db]
	host = "{{ .Db.Host }}"
	port = {{ .Db.Port }}
	username = "{{ .Db.Username }}"
	password = "{{ .Db.Password }}"
	schema = "{{ .Db.Schema }}"
	migration-path = "{{ .Db.MigrationPath }}"
[connection]
  host = "0.0.0.0"
  port = 28300
  rendezvous = "rendezvous"
  peers = {{ .Connection.BootstrapPeers }}
`

var configTemplate *template.Template

func init() {
	var err error

	tmpl := template.New("dheartConfigTemplate")

	if configTemplate, err = tmpl.Parse(defaultConfigTemplate); err != nil {
		panic(err)
	}
}

func WriteConfigFile(configFilePath string, config HeartConfig) {
	var buffer bytes.Buffer

	if err := configTemplate.Execute(&buffer, config); err != nil {
		panic(err)
	}

	ioutil.WriteFile(configFilePath, buffer.Bytes(), 0600)
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}
