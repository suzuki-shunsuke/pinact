package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/suzuki-shunsuke/pinact/pkg/controller/run"
)

func main() {
	if err := core(); err != nil {
		log.Fatal(err)
	}
}

func core() error {
	if err := gen(&run.Config{}, "json-schema/pinact.json"); err != nil {
		return err
	}
	return nil
}

func gen(input interface{}, p string) error {
	f, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create a file %s: %w", p, err)
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	s := jsonschema.Reflect(input)
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("mashal schema as JSON: %w", err)
	}
	if err := os.WriteFile(p, []byte(strings.ReplaceAll(string(b), "http://json-schema.org", "https://json-schema.org")+"\n"), 0o644); err != nil { //nolint:gosec,mnd
		return fmt.Errorf("write JSON Schema to %s: %w", p, err)
	}
	return nil
}
