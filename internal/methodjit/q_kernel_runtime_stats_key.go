package methodjit

type qKernelExecutionKey struct {
	source        string
	kernel        string
	shape         string
	pipelineShape string
	route         string
	outcome       string
	reasonCode    string
}
