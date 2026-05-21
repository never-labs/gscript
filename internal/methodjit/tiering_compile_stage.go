//go:build darwin && arm64

package methodjit

type tier2CompileStage struct {
	name string
	run  func() error
}

func runTier2CompileStages(trace *Tier2Trace, stages []tier2CompileStage) error {
	for _, stage := range stages {
		if trace == nil {
			if err := stage.run(); err != nil {
				return err
			}
			continue
		}
		scope := beginPhaseScope(stage.name)
		err := stage.run()
		trace.PipelineStages = append(trace.PipelineStages, scope.timing(err))
		if err != nil {
			return err
		}
	}
	return nil
}
