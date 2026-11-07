//go:generate openapi-generator generate -i openapi-spec.yml -g go -o openapi --git-host github.com --git-user-id gholamtaksir --git-repo-id Harp/openapi --additional-properties=disallowAdditionalPropertiesIfNotPresent=false

package main

import (
	"log"

	"github.com/gholamtaksir/Harp/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalf("%+v\n", err)
	}
}
