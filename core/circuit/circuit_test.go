package circuit

import (
	"testing"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type stubLoad struct {
	circuit    api.Circuit
	power      float64
	current    float64
	minPower   float64
	minCurrent float64
	priority   int
}

func (l *stubLoad) GetChargePower() float64     { return l.power }
func (l *stubLoad) GetMaxPhaseCurrent() float64 { return l.current }
func (l *stubLoad) GetCircuit() api.Circuit     { return l.circuit }
func (l *stubLoad) EffectivePriority() int      { return l.priority }
func (l *stubLoad) EffectiveMinPower() float64  { return l.minPower }
func (l *stubLoad) GetMinCurrent() float64      { return l.minCurrent }

type circuitTest struct {
	// current values for parent, circuit 1, circuit 2
	p, c1, c2 float64
	// old/new demand values and allowed result
	old, new, res float64
}

func circuitTests() []circuitTest {
	return []circuitTest{
		// no load
		{0, 0, 0, 0, 0, 0}, // =
		{0, 0, 0, 0, 1, 1}, // +
		{0, 0, 0, 0, 2, 1}, // +
		{0, 0, 0, 1, 1, 1}, // =

		// circuit 1 loaded
		{0, 1, 0, 0, 0, 0}, // =
		{0, 1, 0, 0, 1, 0}, // +
		{0, 1, 0, 0, 2, 0}, // +
		{0, 1, 0, 1, 1, 1}, // =
		{0, 1, 0, 2, 1, 1}, // -

		// circuit 1 overloaded
		{0, 2, 0, 0, 0, 0}, // =
		{0, 2, 0, 0, 1, 0}, // +
		{0, 2, 0, 1, 1, 0}, // =
		{0, 2, 0, 2, 2, 1}, // =
		{0, 2, 0, 2, 3, 1}, // +
		{0, 2, 0, 2, 1, 1}, // -

		{0, 1.1, 0, 2, 1, 1}, // -
		{0, 1.1, 0, 1, 0, 0}, // -

		// parent loaded
		{1, 0, 0, 0, 0, 0}, // =
		{1, 0, 0, 0, 1, 0}, // +
		{1, 0, 0, 0, 2, 0}, // +
		{1, 0, 0, 1, 1, 1}, // =
		{1, 0, 0, 2, 1, 1}, // -

		// parent overloaded
		{2, 0, 0, 0, 0, 0}, // =
		{2, 0, 0, 0, 1, 0}, // +
		{2, 0, 0, 1, 1, 0}, // =
		{2, 0, 0, 2, 2, 1}, // =
		{2, 0, 0, 2, 3, 1}, // +
		{2, 0, 0, 2, 1, 1}, // -

		{1.1, 0, 0, 2, 1, 1}, // -
		{1.1, 0, 0, 1, 0, 0}, // -

		// negative load
		{-1, -1, 0, 0, 2, 2}, // +
	}
}

func TestCircuitPower(t *testing.T) {
	log := util.NewLogger("foo")

	circ := func(t *testing.T, ctrl *gomock.Controller, maxP float64) (*Circuit, *api.MockMeter) {
		m := api.NewMockMeter(ctrl)
		c, err := New(log, "foo", 0, maxP, m, 0)
		require.NoError(t, err)
		return c, m
	}

	for _, tc := range circuitTests() {
		ctrl := gomock.NewController(t)

		pc, pm := circ(t, ctrl, 1)
		c1, cm1 := circ(t, ctrl, 1)
		c2, cm2 := circ(t, ctrl, 1)

		c1.setParent(pc)
		c2.setParent(pc)

		// update meters
		pm.EXPECT().CurrentPower().Return(tc.p, nil)
		cm1.EXPECT().CurrentPower().Return(tc.c1, nil)
		cm2.EXPECT().CurrentPower().Return(tc.c2, nil)
		require.NoError(t, pc.Update(nil))

		assert.Equal(t, tc.res, c1.ValidatePower(tc.old, tc.new), tc)

		ctrl.Finish()
	}
}

func TestCircuitCurrents(t *testing.T) {
	log := util.NewLogger("foo")

	type combined struct {
		*api.MockMeter
		*api.MockPhaseCurrents
	}
	circ := func(t *testing.T, ctrl *gomock.Controller, maxC float64) (*Circuit, combined) {
		m := combined{
			api.NewMockMeter(ctrl),
			api.NewMockPhaseCurrents(ctrl),
		}
		c, err := New(log, "foo", maxC, 0, m, 0)
		require.NoError(t, err)
		return c, m
	}

	for _, tc := range circuitTests() {
		ctrl := gomock.NewController(t)

		pc, pm := circ(t, ctrl, 1)
		c1, cm1 := circ(t, ctrl, 1)
		c2, cm2 := circ(t, ctrl, 1)

		c1.setParent(pc)
		c2.setParent(pc)

		// update meters
		pm.MockMeter.EXPECT().CurrentPower().AnyTimes().Return(0.0, nil)
		cm1.MockMeter.EXPECT().CurrentPower().AnyTimes().Return(0.0, nil)
		cm2.MockMeter.EXPECT().CurrentPower().AnyTimes().Return(0.0, nil)
		pm.MockPhaseCurrents.EXPECT().Currents().Return(tc.p, tc.p, tc.p, nil)
		cm1.MockPhaseCurrents.EXPECT().Currents().Return(tc.c1, tc.c1, tc.c1, nil)
		cm2.MockPhaseCurrents.EXPECT().Currents().Return(tc.c2, tc.c2, tc.c2, nil)
		require.NoError(t, pc.Update(nil))

		assert.Equal(t, tc.res, c1.ValidateCurrent(tc.old, tc.new), tc)

		ctrl.Finish()
	}
}

func TestCircuitPriorityAllocation(t *testing.T) {
	log := util.NewLogger("prio")

	circuit, err := New(log, "prio", 0, 6000, nil, 0)
	require.NoError(t, err)

	high := &stubLoad{circuit: circuit, power: 3000, current: 16, priority: 1}
	low := &stubLoad{circuit: circuit, power: 3000, current: 16, priority: 0}

	require.NoError(t, circuit.Update([]api.CircuitLoad{high, low}))

	assert.Equal(t, 4000.0, circuit.ValidatePowerWithPriority(high, high.power, 4000))

	high.power = 4000
	low.power = 2000

	require.NoError(t, circuit.Update([]api.CircuitLoad{high, low}))

	assert.Equal(t, 2000.0, circuit.ValidatePowerWithPriority(low, low.power, 3000))
}

func TestCircuitPriorityCurrent(t *testing.T) {
	log := util.NewLogger("prio")

	circuit, err := New(log, "prio", 32, 0, nil, 0)
	require.NoError(t, err)

	high := &stubLoad{circuit: circuit, current: 16, power: 3000, priority: 1, minCurrent: 0}
	low := &stubLoad{circuit: circuit, current: 16, power: 3000, priority: 0, minCurrent: 0}

	require.NoError(t, circuit.Update([]api.CircuitLoad{high, low}))

	assert.Equal(t, 20.0, circuit.ValidateCurrentWithPriority(high, high.current, 20))

	high.current = 20
	low.current = 12

	require.NoError(t, circuit.Update([]api.CircuitLoad{high, low}))

	assert.Equal(t, 12.0, circuit.ValidateCurrentWithPriority(low, low.current, 16))
}
