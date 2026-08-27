package contract

import "embed"

//go:embed schemas/*.json
var schemas embed.FS

func Schema(name string) ([]byte, error) {
	return schemas.ReadFile("schemas/" + name)
}
