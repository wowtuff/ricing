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
	reg.Register(&ReadFileTool{})
	reg.Register(&ApplyPatchTool{})
	reg.Register(&SystemInfoTool{})
	reg.Register(&ServiceLogsTool{})

	return reg
}
