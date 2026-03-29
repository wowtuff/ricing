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
	reg.Register(&ColorModeTool{})
	reg.Register(UpdatePlanTool{})
	reg.Register(RequestUserInputTool{})

	return reg
}
