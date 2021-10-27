package config

import (
	"bytes"
	"io/ioutil"
	"text/template"
)

const defaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

home-dir = "{{ .HomeDir }}"
use-on-memory = {{ .UseOnMemory }}
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
