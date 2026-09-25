package main

// Cenário da lição asin().
//
// asin_accelerometer_x_mps2{device}: aceleração medida no eixo X de um
// inclinômetro (acelerômetro) parado. Parado, ele só "sente" a gravidade:
//     a_x = 9.81 * sin(inclinação)
// então inclinação = asin(a_x / 9.81).
//   device="guindaste": inclina entre -60° e +60° (período 3 min), sensor limpo.
//   device="betoneira": inclinação ~30°, mas a cada 5 min passa 40s VIBRANDO:
//       a leitura ganha picos de até ±6 m/s², que passam de 9.81 -> asin() = NaN.
// asin_gravity_mps2: a constante g = 9.81 (para dividir na query).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "asin",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"asin_accelerometer_x_mps2",
				"Aceleração no eixo X do inclinômetro (m/s²). Parado: 9.81*sin(inclinação).",
				prometheus.GaugeValue, []string{"device"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					crane := 9.81 * math.Sin(60*math.Pi/180*math.Sin(2*math.Pi*t/180))
					mixer := 9.81*math.Sin(math.Pi/6) + Noise(0.1)
					if math.Mod(t, 300) < 40 {
						mixer = 9.81*math.Sin(math.Pi/3) + Noise(6) // vibração + inclinação 60°
					}
					return []Sample{
						{Labels: []string{"guindaste"}, Value: crane},
						{Labels: []string{"betoneira"}, Value: mixer},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"asin_gravity_mps2", "Aceleração da gravidade (m/s²).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample { return []Sample{{Value: 9.81}} },
			))
		},
	})
}
