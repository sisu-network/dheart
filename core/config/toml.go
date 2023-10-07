package config

import (
	"bytes"
	"os"
	"text/template"
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
{{ range .Connection.Peers }}
  [[connection.peers]]
    address = "{{ .Address }}"
    pubkey = "{{ .PubKey }}"
    pubkey_type = "{{ .PubKeyType }}"
{{end}}
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

	os.WriteFile(configFilePath, buffer.Bytes(), 0600)
}
