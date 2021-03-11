package envi

import (
	"fmt"
	"github.com/catmullet/envi/internal"
)

var (
	Production = internal.Production
	Developer  = internal.Developer
)

func SetEnv(env internal.Environment) error {
	var envi = &internal.Envi{}
	if _, err := envi.Load(); err != nil {
		return fmt.Errorf("failed to set env variables: %w", err)
	}
	if err := envi.ExportVars(env); err != nil {
		return fmt.Errorf("failed to set env variables: %w", err)
	}
	return nil
}
