package files

import (
	"embed"
	"fmt"
)

func GetTemplateBytes(templates embed.FS, fileName string) ([]byte, error) {
	tmplBytes, err := templates.ReadFile(fmt.Sprintf("templates/%s.go.tmpl", fileName))
	if err != nil {
		return nil, err
	}
	return tmplBytes, nil
}
