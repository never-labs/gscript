package bind

import (
	"math"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func scientificInteropInterp(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	installTestModule(interp, "control", runtime.TableValue(BuildControl()))
	installTestModule(interp, "linalg", runtime.TableValue(BuildLinalg()))
	installTestModule(interp, "matrix", runtime.TableValue(BuildMatrix()))
	installTestModule(interp, "ode", runtime.TableValue(BuildODE(interp.CallFunction)))
	installTestModule(interp, "stats", runtime.TableValue(BuildStats()))
	execOnInterp(t, interp, src)
	return interp
}

func TestScientificInteropControlLQR2AcceptsLinalgMatrix(t *testing.T) {
	interp := scientificInteropInterp(t, `
A := linalg.matrix(2, 2, {0, 1, 9.81, -0.1})
B := linalg.matrix(2, 1, {0, 1})
Q := linalg.matrix(2, 2, {10, 0, 0, 1})
gain := control.lqr2(A, B, Q, 1)
gainNorm := linalg.norm(gain)
`)
	gain := interp.GetGlobal("gain")
	if !gain.IsDenseArray() {
		t.Fatalf("control.lqr2(linalg.matrix(...)) returned %s, want dense array", gain.TypeName())
	}
	xs, ok := gain.DenseArray().F64()
	if !ok || len(xs) != 2 {
		t.Fatalf("gain = %#v, want f64[2]", gain)
	}
	for i, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			t.Fatalf("gain[%d] = %v, want finite", i, x)
		}
	}
	if got := interp.GetGlobal("gainNorm"); !got.IsNumber() || got.Number() <= 0 {
		t.Fatalf("gainNorm = %v, want positive number", got)
	}
}

func TestScientificInteropStatsAcceptODERK4StateVector(t *testing.T) {
	interp := scientificInteropInterp(t, `
func constantVelocity(state) {
	return {2, 4, 6}
}
next := ode.rk4(constantVelocity, {1, 2, 3}, 0.5)
mean := stats.mean(next)
variance := stats["var"](next)
`)
	if got := interp.GetGlobal("next"); !got.IsDenseArray() {
		t.Fatalf("ode.rk4 returned %s, want dense array", got.TypeName())
	}
	assertFloat(t, interp.GetGlobal("mean"), 4)
	assertFloat(t, interp.GetGlobal("variance"), 8.0/3.0)
}

func TestScientificInteropLinalgAcceptODERK4DenseArrayState(t *testing.T) {
	interp := scientificInteropInterp(t, `
func constantVelocity(state) {
	return {2, 4, 6}
}
next := ode.rk4(constantVelocity, {1, 2, 3}, 0.5)
norm := linalg.norm(next)
dot := linalg.dot(next, next)
`)
	assertFloat(t, interp.GetGlobal("norm"), math.Sqrt(56))
	assertFloat(t, interp.GetGlobal("dot"), 56)
}

func TestScientificInteropLinalgAcceptsMatrixDense(t *testing.T) {
	interp := scientificInteropInterp(t, `
m := matrix.dense(2, 2)
matrix.setf(m, 0, 0, 1)
matrix.setf(m, 0, 1, 2)
matrix.setf(m, 1, 0, 3)
matrix.setf(m, 1, 1, 4)
got := linalg.get(m, 2, 1)
product := linalg.matmul(m, linalg.eye(2))
productValue := linalg.get(product, 2, 2)
`)
	assertFloat(t, interp.GetGlobal("got"), 3)
	assertFloat(t, interp.GetGlobal("productValue"), 4)
}

func TestScientificInteropODEClosedLoopUsesPolicyMetadata(t *testing.T) {
	interp := scientificInteropInterp(t, `
K := linalg.row(2.0, 0.5)
policy := control.policy(K, {limit: 10.0, state_names: {"theta", "omega"}, wrap_angles: {"theta"}})
state := {0.2, 0.0}

func plant(x, u) {
	return {
		theta: x.omega,
		omega: -0.1 * x.omega + u,
	}
}

func observe_energy(x, step, t) {
	return {
		energy: x.theta * x.theta + x.omega * x.omega,
	}
}

closed := ode.closed_loop(plant, state, policy, {dt: 0.05, steps: 8, trajectory: true, observe: observe_energy})

func manual_dynamics(x) {
	u := control.apply(policy, x)
	return plant(x, u)
}

manual := ode.solve(manual_dynamics, state, {dt: 0.05, steps: 8, trajectory: true, state_names: {"theta", "omega"}, named_state: true, wrap_angles: {"theta"}, observe: observe_energy})

closed_theta := closed.final_state.theta
manual_theta := manual.final_state.theta
closed_omega := closed.final_state.omega
manual_omega := manual.final_state.omega
closed_energy_summary := stats.describe_fields(closed.observed).energy
`)
	assertNear(t, interp.GetGlobal("closed_theta"), interp.GetGlobal("manual_theta").Number(), 1e-12)
	assertNear(t, interp.GetGlobal("closed_omega"), interp.GetGlobal("manual_omega").Number(), 1e-12)
	if got := interp.GetGlobal("closed_energy_summary").Table().RawGetString("max").Number(); got <= 0 {
		t.Fatalf("closed loop energy max = %.12f, want positive", got)
	}
}
