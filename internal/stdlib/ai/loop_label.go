package ai

const (
	LoopNameReact       = "react"
	LoopNameSimple      = "simple"
	LoopNamePlanExecute = "plan_execute"
	LoopNameReflect     = "reflect"
)

// LoopFunctionLabel returns the runtime function label for a loop entrypoint.
func LoopFunctionLabel(namespace, name string) string {
	return namespace + "." + name
}
