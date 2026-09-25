package main

// Cenário da lição time().
//
// time() não precisa de métrica nenhuma, mas ele brilha quando é comparado com
// métricas cujo VALOR é um timestamp Unix (segundos desde 1970, UTC).
//
// 1) time_backup_last_success_timestamp_seconds{backup} -> "quando foi o último backup OK?"
//    db-backup      roda a cada 3 min  -> idade vai de 0 a 180s (dente-de-serra)
//    files-backup   roda a cada 10 min -> idade vai de 0 a 600s
//    legacy-backup  QUEBRADO: último sucesso foi ontem 00:00 UTC -> idade entre 1 e 2 dias
//
// 2) time_process_start_time_seconds{pod} -> imita process_start_time_seconds (client_golang):
//    api-0 reinicia a cada 6 min (crashloop) -> uptime 0..360s
//    api-1 no ar há ~3 dias                 -> uptime 3 a 4 dias
//
// 3) time_probe_ssl_earliest_cert_expiry{domain} -> "quando o certificado expira?"
//    api.lab.local  expira daqui a ~45 dias
//    shop.lab.local expira daqui a ~12 dias
//    old.lab.local  expira daqui a ~3 dias  (deveria alertar!)
//    (as datas são "meia-noite UTC de hoje + N dias", então não mudam durante o dia)

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"time_backup_last_success_timestamp_seconds",
				"Timestamp Unix (s, UTC) do último backup concluído com sucesso.",
				prometheus.GaugeValue, []string{"backup"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"db-backup"}, Value: math.Floor(t/180) * 180},
						{Labels: []string{"files-backup"}, Value: math.Floor(t/600) * 600},
						{Labels: []string{"legacy-backup"}, Value: math.Floor(t/86400)*86400 - 86400},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"time_process_start_time_seconds",
				"Imita process_start_time_seconds: quando o processo subiu (timestamp Unix em s). O pod 'api-0' reinicia a cada 6 min.",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"api-0"}, Value: math.Floor(t/360) * 360},
						{Labels: []string{"api-1"}, Value: math.Floor(t/86400)*86400 - 3*86400},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"time_probe_ssl_earliest_cert_expiry",
				"Timestamp Unix (s, UTC) em que o certificado TLS expira (not after).",
				prometheus.GaugeValue, []string{"domain"},
				func(now time.Time) []Sample {
					midnight := math.Floor(float64(now.Unix())/86400) * 86400
					return []Sample{
						{Labels: []string{"api.lab.local"}, Value: midnight + 45*86400},
						{Labels: []string{"shop.lab.local"}, Value: midnight + 12*86400},
						{Labels: []string{"old.lab.local"}, Value: midnight + 3*86400},
					}
				},
			))
		},
	})
}
