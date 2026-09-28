package config

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	yamlExample = "../../../deploy/config.example.yaml"
	envExample  = "../../../deploy/.env.example"
)

func TestConfigExample_DocumentsEveryFileKey(t *testing.T) {
	raw, err := os.ReadFile(yamlExample)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", yamlExample, err)
	}
	documented := flattenKeys("", doc)

	for _, key := range yamlKeys("", reflect.TypeFor[fileConfig]()) {
		if !slices.Contains(documented, key) {
			t.Errorf("%s does not document config key %q", yamlExample, key)
		}
	}
}

func TestConfigExample_Loads(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	if _, err := Load(yamlExample); err != nil {
		t.Fatalf("Load(%s): %v", yamlExample, err)
	}
}

func TestEnvExample_DocumentsEveryEnvVar(t *testing.T) {
	src, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	example, err := os.ReadFile(envExample)
	if err != nil {
		t.Fatal(err)
	}
	read := regexp.MustCompile(`(?:env\w*|os\.Getenv)\("([A-Z][A-Z0-9_]+)"`).FindAllSubmatch(src, -1)
	if len(read) == 0 {
		t.Fatal("found no env var reads in config.go; the pattern is stale")
	}
	for _, m := range read {
		name := string(m[1])
		if !regexp.MustCompile(`(?m)^#? ?` + name + `=`).Match(example) {
			t.Errorf("%s does not document env var %s", envExample, name)
		}
	}
}

func yamlKeys(prefix string, typ reflect.Type) []string {
	var keys []string
	for f := range typ.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			continue
		}
		key := prefix + name
		if f.Type.Kind() == reflect.Struct {
			keys = append(keys, yamlKeys(key+".", f.Type)...)
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func flattenKeys(prefix string, m map[string]any) []string {
	var keys []string
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			keys = append(keys, flattenKeys(prefix+k+".", sub)...)
			continue
		}
		keys = append(keys, prefix+k)
	}
	return keys
}
