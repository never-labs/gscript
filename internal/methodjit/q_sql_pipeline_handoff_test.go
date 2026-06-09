//go:build darwin && arm64

package methodjit

import "testing"

func TestQSQLKernelRuntimeBackendPlanNormalizesStableHandoff(t *testing.T) {
	plan := QSQLKernelRuntimeBackendPlan(QSQLKernelPipelineRef{
		Shape:         "select/where/project",
		PipelineShape: "scan=frame|where=compare_mask:column_literal|filter=index|project=column:1",
		SchemaHash:    "schema-a",
	})
	if !plan.Valid() {
		t.Fatalf("QSQLKernelRuntimeBackendPlan invalid: %#v", plan)
	}
	if plan.Backend != QSQLKernelRuntimeRoute {
		t.Fatalf("Backend = %q, want %q", plan.Backend, QSQLKernelRuntimeRoute)
	}
	if plan.Ref.Kernel != QSQLKernelName {
		t.Fatalf("Ref.Kernel = %q, want %q", plan.Ref.Kernel, QSQLKernelName)
	}
	if plan.Ref.Route != QSQLKernelRuntimeRoute {
		t.Fatalf("Ref.Route = %q, want %q", plan.Ref.Route, QSQLKernelRuntimeRoute)
	}
	if plan.Detail == "" {
		t.Fatalf("Detail empty for normalized qSQL backend plan")
	}
}

func TestQSQLKernelRuntimeDescriptorFromBackendPlanPreservesPipelineShape(t *testing.T) {
	plan := QSQLKernelBackendPlan{
		Backend: "typed_runtime_qsql_columnar",
		Ref: QSQLKernelPipelineRef{
			Kernel:        "CustomQSQLKernel",
			Shape:         "select/by/aggregate",
			PipelineShape: "scan=frame|group=column:1|aggregate=sum:1",
			Route:         QSQLKernelRuntimeRoute,
			SchemaHash:    "schema-b",
		},
	}
	descriptor := QSQLKernelRuntimeDescriptorFromBackendPlan(plan)
	if descriptor.Source != QSQLKernelRuntimeSource ||
		descriptor.Kind != "runtime_kernel" ||
		descriptor.Kernel != "CustomQSQLKernel" ||
		descriptor.Shape != "select/by/aggregate" ||
		descriptor.PipelineShape != "scan=frame|group=column:1|aggregate=sum:1" ||
		descriptor.Route != "typed_runtime_qsql_columnar" ||
		descriptor.Outcome != "supported" {
		t.Fatalf("descriptor = %#v, want backend route and qSQL pipeline fields preserved", descriptor)
	}
}
