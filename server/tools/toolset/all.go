package toolset

import (
	"github.com/wowtuff/ricing/tools"
)

func NewDefaultRegistry() *tools.Registry {
	reg := tools.NewRegistry()

	reg.Register(MultiplyTool{})
	reg.Register(NotifyTool{})
	reg.Register(&CmdTool{})
	reg.Register(&InstallPackageTool{})

	return reg
}
