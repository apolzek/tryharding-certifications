package main

// Cenário da lição acos().
//
// Uma escada de 5 m encostada numa parede. Um sensor mede a distância do pé da
// escada até a parede. O ângulo da escada com o chão é:
//     θ = acos(distância / comprimento)
// acos_ladder_base_distance_meters{ladder}:
//   ladder="segura":  pé desliza devagar entre 1.0 e 2.5 m (período 4 min) -> 60°..78°
//   ladder="caindo":  pé escorrega de 1 m até 5.5 m a cada 3 min. Depois de 5 m a
//                     razão passa de 1 -> acos() = NaN (geometria impossível: a escada caiu).
// acos_ladder_length_meters{ladder}: 5 m (constante).
//
// Caso real (IoT/rastreamento): exporter de GPS de frota.
// acos_vehicle_latitude_degrees{vehicle} / acos_vehicle_longitude_degrees{vehicle}:
//   vehicle="caminhao-1": vai do CD em São Paulo (-23.55, -46.63) até Campinas
//       (-22.91, -47.06) e volta, a cada 4 min (~84 km no ponto mais distante).
//   vehicle="moto-2": parada no CD (distância 0).
// Distância do CD = 6371 * acos(lei esférica dos cossenos).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "acos",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"acos_ladder_base_distance_meters",
				"Distância do pé da escada até a parede (m).",
				prometheus.GaugeValue, []string{"ladder"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					safe := Wave(t, 240, 1.75, 0.75, 0)
					falling := 1 + 4.5*Saw(t, 180)
					return []Sample{
						{Labels: []string{"segura"}, Value: safe},
						{Labels: []string{"caindo"}, Value: falling},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"acos_ladder_length_meters", "Comprimento da escada (m).",
				prometheus.GaugeValue, []string{"ladder"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"segura"}, Value: 5}, {Labels: []string{"caindo"}, Value: 5}}
				},
			))
			pos := func(now time.Time) float64 {
				return 1 - math.Abs(2*Saw(float64(now.UnixNano())/1e9, 240)-1) // 0 -> 1 -> 0
			}
			reg.MustRegister(NewFunc(
				"acos_vehicle_latitude_degrees", "Latitude do veículo (graus).",
				prometheus.GaugeValue, []string{"vehicle"},
				func(now time.Time) []Sample {
					return []Sample{
						{Labels: []string{"caminhao-1"}, Value: -23.55 + pos(now)*(-22.91+23.55)},
						{Labels: []string{"moto-2"}, Value: -23.55},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"acos_vehicle_longitude_degrees", "Longitude do veículo (graus).",
				prometheus.GaugeValue, []string{"vehicle"},
				func(now time.Time) []Sample {
					return []Sample{
						{Labels: []string{"caminhao-1"}, Value: -46.63 + pos(now)*(-47.06+46.63)},
						{Labels: []string{"moto-2"}, Value: -46.63},
					}
				},
			))
		},
	})
}
