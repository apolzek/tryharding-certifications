package main

// Cenário da lição pi().
//
// 1) pi_turbine_rotor_rpm{turbine}: rotação do rotor de uma turbina eólica
//    (rotações por minuto). Varia com o vento entre 6 e 16 rpm (período 4 min).
//    pi_turbine_blade_length_meters{turbine}: comprimento da pá (raio) = 60 m.
//    Velocidade da ponta da pá (m/s) = 2*pi()*r*rpm/60  -> 16 rpm ≈ 100 m/s!
//    Área varrida = pi()*r^2 ≈ 11 310 m².
// 2) pi_pipe_diameter_meters{pipe} e pi_pipe_flow_velocity_mps{pipe}: tubulação
//    de água. Vazão (m³/s) = pi()*(d/2)^2 * velocidade.
//    pipe="adutora" d=0.5 m; pipe="ramal" d=0.1 m. Velocidade 1..3 m/s.

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "pi",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"pi_turbine_rotor_rpm", "Rotação do rotor (rpm).",
				prometheus.GaugeValue, []string{"turbine"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{
						{Labels: []string{"T1"}, Value: Wave(t, 240, 11, 5, 0) + Noise(0.2)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"pi_turbine_blade_length_meters", "Comprimento da pá = raio do rotor (m).",
				prometheus.GaugeValue, []string{"turbine"},
				func(now time.Time) []Sample { return []Sample{{Labels: []string{"T1"}, Value: 60}} },
			))
			reg.MustRegister(NewFunc(
				"pi_pipe_diameter_meters", "Diâmetro interno da tubulação (m).",
				prometheus.GaugeValue, []string{"pipe"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"adutora"}, Value: 0.5}, {Labels: []string{"ramal"}, Value: 0.1}}
				},
			))
			reg.MustRegister(NewFunc(
				"pi_pipe_flow_velocity_mps", "Velocidade da água na tubulação (m/s).",
				prometheus.GaugeValue, []string{"pipe"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{
						{Labels: []string{"adutora"}, Value: Wave(t, 180, 2, 1, 0)},
						{Labels: []string{"ramal"}, Value: Wave(t, 180, 2, 1, 1.5)},
					}
				},
			))
		},
	})
}
